package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKey = "gsk_SUPERSECRETKEY_should_never_leak_1234567890"

// newTestClient builds a client pointed at a mock server, bypassing the env so
// tests are hermetic.
func newTestClient(baseURL string) *GroqClient {
	return &GroqClient{
		apiKey:  testKey,
		model:   "openai/gpt-oss-20b",
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

func TestGroqDiagnose_RequestShapeAndParse(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)

		// Return a valid Diagnosis as the assistant message content.
		content := `{"engine":"groq","performance":[{"title":"Memoria","detail":"alta","action":"cierra apps","severity":"high"}],"quick_wins":[{"title":"browser","detail":"2GB","action":"limpia","severity":"low"}],"review":[],"reclaimable_safe_bytes":2147483648,"context":{"disk_used_pct":80,"disk_free_bytes":100,"disk_total_bytes":500}}`
		resp := map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"role": "assistant", "content": content}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	d, err := c.GroqDiagnose(context.Background(), AISnapshot{DiskUsedPct: 80})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Request validation.
	if gotAuth != "Bearer "+testKey {
		t.Fatalf("Authorization = %q, want Bearer <key>", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotBody["model"] != "openai/gpt-oss-20b" {
		t.Fatalf("model = %v, want openai/gpt-oss-20b", gotBody["model"])
	}
	rf, ok := gotBody["response_format"].(map[string]any)
	if !ok || rf["type"] != "json_object" {
		t.Fatalf("response_format = %v, want {type: json_object}", gotBody["response_format"])
	}

	// Response parsing.
	if d.Engine != EngineGroq {
		t.Fatalf("engine = %q, want groq", d.Engine)
	}
	if len(d.Performance) != 1 || d.Performance[0].Title != "Memoria" {
		t.Fatalf("performance not parsed: %+v", d.Performance)
	}
	if d.ReclaimableSafeBytes != 2147483648 {
		t.Fatalf("reclaimable = %d, want 2147483648", d.ReclaimableSafeBytes)
	}
}

func TestGroqDiagnose_InvalidContentIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "sorry, I can't do JSON today"}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GroqDiagnose(context.Background(), AISnapshot{}); err == nil {
		t.Fatal("expected error for non-JSON content, got nil")
	}
}

func TestGroqDiagnose_InvalidSeverityFailsSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// severity "urgent" is not in the allowed set -> must fail validation.
		content := `{"engine":"groq","performance":[{"title":"x","detail":"y","action":"z","severity":"urgent"}],"quick_wins":[],"review":[],"reclaimable_safe_bytes":0,"context":{"disk_used_pct":0,"disk_free_bytes":0,"disk_total_bytes":0}}`
		resp := map[string]any{"choices": []map[string]any{{"message": map[string]any{"content": content}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GroqDiagnose(context.Background(), AISnapshot{}); err == nil {
		t.Fatal("expected schema validation error for bad severity, got nil")
	}
}

// TestGroqDiagnose_KeyRedactedOnHTTPError is the secret-redaction guarantee:
// when the server errors, the returned error must never contain the API key.
func TestGroqDiagnose_KeyRedactedOnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the key back in the error body to simulate a provider that
		// reflects the Authorization header — the worst case for leakage.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "invalid api key: "+r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GroqDiagnose(context.Background(), AISnapshot{})
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("API key leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "REDACTED") {
		t.Fatalf("expected redaction marker in error, got: %v", err)
	}
}

// TestDiagnose_FallsBackToHeuristicWithoutKey verifies the orchestrator never
// fails when no key is present and never calls out.
func TestDiagnose_FallsBackToHeuristicWithoutKey(t *testing.T) {
	t.Setenv(envAPIKey, "")
	d := Diagnose(AISnapshot{DiskUsedPct: 90, DiskTotalBytes: 500 * gb, DiskFreeBytes: 50 * gb}, Options{})
	if d.Engine != EngineHeuristic {
		t.Fatalf("engine = %q, want heuristic (no key set)", d.Engine)
	}
}

func TestDiagnose_NoAIForcesHeuristic(t *testing.T) {
	// Even with a key, --no-ai must skip the network entirely.
	t.Setenv(envAPIKey, testKey)
	d := Diagnose(AISnapshot{DiskTotalBytes: 500 * gb}, Options{NoAI: true})
	if d.Engine != EngineHeuristic {
		t.Fatalf("engine = %q, want heuristic (NoAI set)", d.Engine)
	}
}
