package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/synth"
)

func init() {
	register("review", "list stale hunches, orphans and unplaced aesthetic atoms (read-only) [--stale-days N]", cmdReview)
}

type reviewResult struct{ synth.Review }

func (r reviewResult) MarshalJSON() ([]byte, error) { return json.Marshal(r.Review) }

func (r reviewResult) Text() string {
	var b strings.Builder
	group := func(title string, ids []string) {
		fmt.Fprintf(&b, "%s (%d)\n", title, len(ids))
		for _, id := range ids {
			fmt.Fprintf(&b, "  [[%s]]\n", id)
		}
	}
	group("stale hunches", r.StaleHunches)
	group("orphans", r.Orphans)
	group("unplaced", r.Unplaced)
	return strings.TrimRight(b.String(), "\n")
}

func cmdReview(env *Env, args []string) (any, error) {
	fs := newFlagSet("review")
	days := fs.Int("stale-days", 0, "")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != 0 {
		return nil, errs.New(errs.BadInput, "review takes no arguments", "usage: slowhunch review [--stale-days N]")
	}
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "stale-days" {
			set = true
		}
	})
	v, cfg, err := env.openVault()
	if err != nil {
		return nil, err
	}
	n := cfg.Review.StaleDays
	if set {
		n = *days
	}
	if n < 1 {
		return nil, errs.New(errs.BadInput, "--stale-days must be >= 1", "usage: slowhunch review [--stale-days N]")
	}
	ix, _, err := v.LoadIndex()
	if err != nil {
		return nil, err
	}
	r, err := synth.RunReview(v, ix, env.Now(), n)
	if err != nil {
		return nil, err
	}
	return reviewResult{r}, nil
}
