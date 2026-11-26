package reporter

import (
	"errors"
	"os"
	"testing"
)

// TestMain disables the system opener for the whole package, so no test can
// open a file on the machine running it.
func TestMain(m *testing.M) {
	OpenFn = func(string) error { return errors.New("opener disabled in tests") }
	os.Exit(m.Run())
}
