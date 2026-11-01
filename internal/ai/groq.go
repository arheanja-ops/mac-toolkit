package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Environment configuration (F2). The API key is read ONLY from the
// environment and never logged, persisted, or placed in the repo.
const (
	envAPIKey  = "GROQ_API_KEY"
	envModel   = "GROQ_MODEL"
	envBaseURL = "GROQ_BASE_URL"

	defaultModel   = "openai/gpt-oss-20b"
	defaultBaseURL = "https://api.groq.com/openai/v1"
	defaultTimeout = 20 * time.Second
)

// systemPrompt instructs the model to act as a macOS optimization expert, to
// prioritize by impact, to distinguish system state from junk, to never
// recommend deleting danger-risk items, and to answer ONLY the Diagnosis JSON.
const systemPrompt = `Eres un experto en optimización de macOS. Recibes un snapshot agregado y anónimo de un Mac (sin rutas, sin nombres de archivo, sin PIDs).
Produce un diagnóstico priorizado por impacto. Distingue el estado del sistema (uso de disco, memoria, térmico) de la basura limpiable.
NUNCA recomiendes borrar elementos cuyo riesgo sea "danger"; esos van a revisión manual.
Responde EXCLUSIVAMENTE con un objeto JSON que cumpla este esquema, sin texto adicional:
{"engine":"groq","performance":[{"title":"","detail":"","action":"","severity":"high|medium|low"}],"quick_wins":[{"title":"","detail":"","action":"","severity":"low"}],"review":[{"title":"","detail":"","action":"","severity":"high|medium"}],"reclaimable_safe_bytes":0,"context":{"disk_used_pct":0,"disk_free_bytes":0,"disk_total_bytes":0}}`

// GroqClient is a minimal OpenAI-compatible chat client built on net/http. It
// holds no secret beyond the process environment it reads at construction.
type GroqClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// NewGroqClient builds a client from the environment. ok is false when no API
// key is set, letting the caller fall straight through to the heuristic.
func NewGroqClient() (c *GroqClient, ok bool) {
	key := strings.TrimSpace(os.Getenv(envAPIKey))
	if key == "" {
		return nil, false
	}
	return &GroqClient{
		apiKey:  key,
		model:   envOr(envModel, defaultModel),
		baseURL: strings.TrimRight(envOr(envBaseURL, defaultBaseURL), "/"),
		http:    &http.Client{Timeout: defaultTimeout},
	}, true
}

// chat request/response types mirror the OpenAI chat-completions shape we use.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	ResponseFormat map[string]string `json:"response_format"`
	Temperature    float64           `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// GroqDiagnose sends the snapshot and parses the model's JSON into a Diagnosis.
// Any transport, HTTP, or schema error is returned (with the API key redacted)
// so the orchestrator can fall back to the heuristic.
func (c *GroqClient) GroqDiagnose(ctx context.Context, snap AISnapshot) (Diagnosis, error) {
	body, err := c.buildRequestBody(snap)
	if err != nil {
		return Diagnosis{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Diagnosis{}, c.redact(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		// The URL in a transport error can embed credentials in edge cases;
		// redact before it reaches any log or the caller.
		return Diagnosis{}, c.redact(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Diagnosis{}, c.redact(err)
	}
	if resp.StatusCode != http.StatusOK {
		return Diagnosis{}, c.redactf("groq: status %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	return c.parseDiagnosis(raw)
}

// buildRequestBody serializes the snapshot as the user message and assembles
// the chat request with forced JSON output.
func (c *GroqClient) buildRequestBody(snap AISnapshot) ([]byte, error) {
	userJSON, err := json.Marshal(snap)
	if err != nil {
		return nil, c.redact(err)
	}
	reqObj := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Snapshot del sistema:\n" + string(userJSON)},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0,
	}
	body, err := json.Marshal(reqObj)
	if err != nil {
		return nil, c.redact(err)
	}
	return body, nil
}

// parseDiagnosis extracts the assistant message and validates it against the
// Diagnosis schema. A missing choice or malformed content is an error so the
// caller falls back to the heuristic rather than showing a half-baked result.
func (c *GroqClient) parseDiagnosis(raw []byte) (Diagnosis, error) {
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Diagnosis{}, c.redactf("groq: decode envelope: %v", err)
	}
	if len(cr.Choices) == 0 {
		return Diagnosis{}, c.redactf("groq: empty choices")
	}

	content := strings.TrimSpace(cr.Choices[0].Message.Content)
	var d Diagnosis
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Diagnosis{}, c.redactf("groq: content is not a valid Diagnosis: %v", err)
	}
	if !validDiagnosis(d) {
		return Diagnosis{}, c.redactf("groq: Diagnosis failed schema validation")
	}

	// Trust the model's text, but stamp the engine ourselves so the field is
	// authoritative regardless of what the model echoed.
	d.Engine = EngineGroq
	return d, nil
}

// validDiagnosis enforces the minimal schema contract: every insight must carry
// a title and a severity drawn from the known set.
func validDiagnosis(d Diagnosis) bool {
	for _, group := range [][]Insight{d.Performance, d.QuickWins, d.Review} {
		for _, in := range group {
			if strings.TrimSpace(in.Title) == "" || !validSeverity(in.Severity) {
				return false
			}
		}
	}
	return true
}

func validSeverity(s string) bool {
	switch s {
	case SevHigh, SevMed, SevLow:
		return true
	default:
		return false
	}
}

// redact wraps an error with the API key masked.
func (c *GroqClient) redact(err error) error {
	if err == nil {
		return nil
	}
	return c.redactf("groq: %v", err)
}

// redactf formats an error and strips any occurrence of the API key, so the
// secret can never leak into logs or the returned error chain.
func (c *GroqClient) redactf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if c.apiKey != "" {
		msg = strings.ReplaceAll(msg, c.apiKey, "***REDACTED***")
	}
	return fmt.Errorf("%s", msg)
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
