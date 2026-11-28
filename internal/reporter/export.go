package reporter

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/arheanja-ops/mac-toolkit/internal/core"
)

// ExportPrefix starts the name of every exported folder, so an export is easy
// to recognise in Downloads or wherever the user put it.
const ExportPrefix = "mac-toolkit-"

// DefaultExportDir is where the TUI proposes to export a report.
const DefaultExportDir = "~/Downloads"

// ExpandHome trims the path and turns a leading "~" or "~/" into home.
// "~otheruser" is left untouched (it is not a path the toolkit resolves).
func ExpandHome(p, home string) string {
	p = strings.TrimSpace(p)
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// ExportReport copies one variant of a saved run (the full report, or its share
// variant) into a NEW folder under dest, named after the run. It returns that
// folder and the copied files. It only reads inside the reports folder, never
// overwrites anything (a repeated export gets a "-2", "-3"… suffix), and the
// copies stay private (0600) like the originals. dest is created if missing.
func ExportReport(runDir, dest string, share bool) (string, []string, error) {
	reports := filepath.Clean(core.ReportsDir()) + string(filepath.Separator)
	runDir = filepath.Clean(runDir)
	if !filepath.IsAbs(runDir) || !strings.HasPrefix(runDir+string(filepath.Separator), reports) || runDir+string(filepath.Separator) == reports {
		return "", nil, fmt.Errorf("refusing to export %q: outside the reports folder", runDir)
	}
	dest = filepath.Clean(strings.TrimSpace(dest))
	if strings.TrimSpace(dest) == "" || dest == "." || !filepath.IsAbs(dest) {
		return "", nil, fmt.Errorf("la carpeta de destino debe ser una ruta absoluta (o empezar por ~/)")
	}
	if strings.HasPrefix(dest+string(filepath.Separator), reports) {
		return "", nil, fmt.Errorf("la carpeta de destino no puede estar dentro de la carpeta de reportes")
	}
	if info, err := os.Stat(dest); err == nil && !info.IsDir() {
		return "", nil, fmt.Errorf("%s no es una carpeta", dest)
	}

	src, name := runDir, ExportPrefix+filepath.Base(runDir)
	if share {
		src, name = filepath.Join(runDir, ShareDir), name+"-share"
	}
	var names []string
	for _, f := range []string{FileMarkdown, FileJSON, FileHTML} {
		if _, err := os.Stat(filepath.Join(src, f)); err == nil {
			names = append(names, f)
		}
	}
	if len(names) == 0 {
		if share {
			return "", nil, fmt.Errorf("el reporte no tiene variante share (es anterior a v1.10)")
		}
		return "", nil, fmt.Errorf("no hay archivos de reporte en %s", runDir)
	}

	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", nil, fmt.Errorf("creando %s: %w", dest, err)
	}
	out, err := newFolder(dest, name)
	if err != nil {
		return "", nil, err
	}
	var files []string
	for _, f := range names {
		p := filepath.Join(out, f)
		if err := copyPrivate(filepath.Join(src, f), p); err != nil {
			return out, files, err
		}
		files = append(files, p)
	}
	return out, files, nil
}

// newFolder creates dest/name, or dest/name-2, -3… when it already exists.
// os.Mkdir fails on an existing folder, so nothing is ever reused.
func newFolder(dest, name string) (string, error) {
	for i := 1; i <= 99; i++ {
		p := filepath.Join(dest, name)
		if i > 1 {
			p = fmt.Sprintf("%s-%d", p, i)
		}
		err := os.Mkdir(p, 0o700)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("creando %s: %w", p, err)
		}
	}
	return "", fmt.Errorf("demasiadas exportaciones de %s en %s", name, dest)
}

// copyPrivate copies one file into a path that must not exist yet.
func copyPrivate(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("leyendo %s: %w", filepath.Base(src), err)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("escribiendo %s: %w", filepath.Base(dst), err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("escribiendo %s: %w", filepath.Base(dst), err)
	}
	return f.Close()
}
