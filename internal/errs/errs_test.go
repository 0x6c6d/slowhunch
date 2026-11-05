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

func TestErrsV2(t *testing.T) {
	codes := map[string]string{
		CapReached: "CAP_REACHED",
		PullFailed: "PULL_FAILED",
		PushFailed: "PUSH_FAILED",
		Behind:     "BEHIND",
	}
	for got, want := range codes {
		if got != want {
			t.Errorf("code %q, want %q", got, want)
		}
	}
	exits := map[string]int{
		CapReached: 2, PullFailed: 1, PushFailed: 1, Behind: 1,
		Conflict: 4, NotFound: 2,
	}
	for code, want := range exits {
		if got := ExitCode(New(code, "m", "h")); got != want {
			t.Errorf("%s: exit %d, want %d", code, got, want)
		}
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
