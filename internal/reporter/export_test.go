package reporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportReportCopiesTheChosenVariant(t *testing.T) {
	withReportsDir(t)
	saved, err := SaveReport(Build(sampleInputs()))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "nueva", "carpeta") // created on demand

	out, files, err := ExportReport(saved.Dir, dest, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(out) != dest || !strings.HasPrefix(filepath.Base(out), ExportPrefix) {
		t.Fatalf("export folder %q must be a new folder under %q", out, dest)
	}
	if len(files) != 3 {
		t.Fatalf("want md, json and html, got %v", files)
	}
	for i, p := range files {
		got, _ := os.ReadFile(p)
		want, _ := os.ReadFile(saved.Full[i])
		if string(got) != string(want) || len(got) == 0 {
			t.Fatalf("%s is not a copy of %s", p, saved.Full[i])
		}
		if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
			t.Fatalf("%s must be private, got %v", p, info.Mode().Perm())
		}
	}

	// A second export of the same run never overwrites the first one.
	out2, _, err := ExportReport(saved.Dir, dest, false)
	if err != nil || out2 == out {
		t.Fatalf("second export must land in a new folder: %q (%v)", out2, err)
	}

	shareOut, shareFiles, err := ExportReport(saved.Dir, dest, true)
	if err != nil || !strings.HasSuffix(shareOut, "-share") || len(shareFiles) != 3 {
		t.Fatalf("share export: %q %v %v", shareOut, shareFiles, err)
	}
	got, _ := os.ReadFile(shareFiles[0])
	want, _ := os.ReadFile(saved.Share[0])
	if string(got) != string(want) {
		t.Fatal("share export must copy the share variant")
	}
}

func TestExportReportRefusesBadInput(t *testing.T) {
	root := withReportsDir(t)
	saved, err := SaveReport(Build(sampleInputs()))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "un-archivo")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	cases := map[string]struct{ run, dest string }{
		"run outside the reports folder": {outside, t.TempDir()},
		"relative destination":           {saved.Dir, "relativa"},
		"empty destination":              {saved.Dir, "  "},
		"destination is a file":          {saved.Dir, file},
		"destination inside the reports": {saved.Dir, root},
	}
	for name, c := range cases {
		if _, _, err := ExportReport(c.run, c.dest, false); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestExpandHome(t *testing.T) {
	for in, want := range map[string]string{
		"~":              "/Users/someone",
		"~/Downloads":    "/Users/someone/Downloads",
		" /tmp/x ":       "/tmp/x",
		"~otro/Download": "~otro/Download",
	} {
		if got := ExpandHome(in, "/Users/someone"); got != want {
			t.Errorf("ExpandHome(%q) = %q, want %q", in, got, want)
		}
	}
}
