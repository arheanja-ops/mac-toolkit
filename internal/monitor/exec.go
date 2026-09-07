package monitor

import "os/exec"

// cmdC builds an exec.Command that forces the C locale.
//
// macOS system tools like `ps` format decimals using the user's locale.
// On locales that use a comma decimal separator (e.g. es_CO), `ps` emits
// "87,1" instead of "87.1", which breaks strconv.ParseFloat and silently
// yields 0.0. Forcing LC_ALL=C guarantees a dot decimal separator so numeric
// parsing is deterministic regardless of the user's regional settings.
func cmdC(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.Env = append(c.Environ(), "LC_ALL=C")
	return c
}
