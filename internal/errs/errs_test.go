package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCode(t *testing.T) {
	cases := map[string]int{
		Conflict: 4, ConflictRev: 4, Exists: 4,
		NotFound: 2, BadInput: 2, Unsupported: 2, TooLong: 2,
		LintFailed: 3, GitMissing: 1, ProviderError: 1,
	}
	for code, want := range cases {
		if got := ExitCode(New(code, "m", "h")); got != want {
			t.Errorf("%s: got %d want %d", code, got, want)
		}
	}
	if ExitCode(nil) != 0 || ExitCode(errors.New("x")) != 1 {
		t.Error("nil/plain")
	}
	if ExitCode(Fatal(NotFound, "m", "h")) != 1 {
		t.Error("fatal")
	}
}

func TestCodeOf(t *testing.T) {
	err := fmt.Errorf("wrap: %w", New(Exists, "m", ""))
	if CodeOf(err) != Exists || CodeOf(errors.New("x")) != "" {
		t.Error("CodeOf")
	}
	if New(Exists, "msg", "").Error() != "EXISTS: msg" {
		t.Error("Error()")
	}
}
