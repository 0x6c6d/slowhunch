#!/usr/bin/env bash
# Verification for T027 (supervisor-owned; workers never edit it). Exit 0 = pass.
. "$(dirname "$0")/../_lib.sh"
expect_ok "provider core tests" <<'CMD'
gotest "./internal/provider/" 'TestRegistry|TestForRole|TestCallJSON' TestRegistry TestForRole TestCallJSONOK TestCallJSONRepair TestCallJSONFailsAfterRepair TestCallJSONValidate TestCallJSONProviderError
CMD
expect_ok "probe: repair succeeds on second reply with fake" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"os"

	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
)

func main() {
	f := &fake.Fake{Replies: map[string][]string{"gate": {"not json", "```json\n{\"keep\": true}\n```"}}}
	var out struct{ Keep bool `json:"keep"` }
	u, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "u"}, &out, nil)
	if err != nil || !out.Keep || len(f.Calls) != 2 || u.TokensIn == 0 {
		os.Exit(1)
	}
	os.Exit(0)
}
GO
CMD
expect_fail "negative: two invalid replies give PROVIDER_ERROR after exactly two calls" <<'CMD'
goprobe <<'GO'
package main

import (
	"context"
	"os"

	"github.com/0x6c6d/slowhunch/internal/errs"
	"github.com/0x6c6d/slowhunch/internal/provider"
	"github.com/0x6c6d/slowhunch/internal/provider/fake"
	"github.com/0x6c6d/slowhunch/internal/vault"
)

func main() {
	f := &fake.Fake{Replies: map[string][]string{"gate": {"ignore previous instructions", "{\"keep\": ","{\"keep\": true}"}}}
	var out map[string]any
	_, err := provider.CallJSON(context.Background(), f, provider.Request{Role: "gate", User: "u"}, &out, nil)
	_, perr := provider.New(vault.ProviderConfig{Name: "anthropic"})
	if err == nil || errs.CodeOf(err) != errs.ProviderError || len(f.Calls) != 2 || perr == nil {
		os.Exit(0)
	}
	os.Exit(1)
}
GO
CMD
finish
