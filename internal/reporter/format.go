package reporter

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxPathRunes is where a displayed path starts to be shortened in the middle.
const maxPathRunes = 60

// reSHA256 matches a full hex digest (docker/ollama style labels carry them).
var reSHA256 = regexp.MustCompile(`[0-9a-fA-F]{64}`)

// ShortLabel makes a label readable: a sha256 digest keeps its first 12
// characters plus "…", and the home folder becomes "~".
func ShortLabel(label, home string) string {
	label = reSHA256.ReplaceAllStringFunc(label, func(h string) string { return h[:12] + "…" })
	return TildePath(label, home)
}

// TildePath replaces the home folder by "~". Only a whole path-segment prefix
// is replaced ("/Users/a" must not match "/Users/ab"). Every occurrence in a
// free-form label is replaced too, so a label that embeds a path stays private.
func TildePath(p, home string) string {
	home = strings.TrimRight(home, "/")
	if home == "" {
		return p
	}
	var b strings.Builder
	for {
		i := strings.Index(p, home)
		if i < 0 {
			b.WriteString(p)
			return b.String()
		}
		end := i + len(home)
		boundary := end == len(p) || p[end] == '/' || p[end] == ' ' || p[end] == '"'
		b.WriteString(p[:i])
		if boundary {
			b.WriteString("~")
		} else {
			b.WriteString(home)
		}
		p = p[end:]
	}
}

// ShortPath shortens a long path for display by dropping its middle segments
// ("~/Library/…/Caches/app"). The model keeps the full path; this is rendering.
func ShortPath(p string) string {
	if utf8.RuneCountInString(p) <= maxPathRunes {
		return p
	}
	segs := strings.Split(p, "/")
	if len(segs) > 5 {
		p = strings.Join(segs[:2], "/") + "/…/" + strings.Join(segs[len(segs)-2:], "/")
	}
	if r := []rune(p); len(r) > maxPathRunes {
		p = "…" + string(r[len(r)-maxPathRunes+1:])
	}
	return p
}
