package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/0x6c6d/slowhunch/internal/errs"
)

const repairSuffix = ". Reply with exactly one JSON object and nothing else."

// stripFence trims whitespace and one optional surrounding Markdown code fence.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") || !strings.HasSuffix(s, "```") || len(s) < 6 {
		return s
	}
	inner := s[3 : len(s)-3]
	nl := strings.IndexByte(inner, '\n')
	if nl < 0 {
		return strings.TrimSpace(inner)
	}
	tag := strings.TrimSpace(inner[:nl])
	if tag != "" && tag != "json" && tag != "JSON" {
		return s
	}
	return strings.TrimSpace(inner[nl+1:])
}

// parseObject requires text to be exactly one JSON object and decodes it into out.
func parseObject(text string, out any) error {
	body := stripFence(text)
	if !strings.HasPrefix(body, "{") {
		return errors.New("reply is not a JSON object")
	}
	dec := json.NewDecoder(strings.NewReader(body))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("extra content after the JSON object")
	}
	raw = bytes.TrimSpace(raw)
	return json.Unmarshal(raw, out)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// CallJSON asks p for one JSON object, decodes it into out and runs validate,
// retrying once with a repair message on a parse or validation failure.
func CallJSON(ctx context.Context, p Provider, req Request, out any, validate func() error) (Usage, error) {
	var u Usage
	attempt := func(r Request) (error, error) {
		resp, err := p.Complete(ctx, r)
		if err != nil {
			return nil, errs.New(errs.ProviderError, "provider call failed: "+err.Error(), "")
		}
		u.TokensIn += resp.TokensIn
		u.TokensOut += resp.TokensOut
		if err := parseObject(resp.Text, out); err != nil {
			return err, nil
		}
		if validate != nil {
			if err := validate(); err != nil {
				return err, nil
			}
		}
		return nil, nil
	}
	bad, fatal := attempt(req)
	if fatal != nil {
		return u, fatal
	}
	if bad == nil {
		return u, nil
	}
	repair := req
	repair.User = req.User + "\n\nYour previous reply was invalid: " + truncate(bad.Error(), 200) + repairSuffix
	bad, fatal = attempt(repair)
	if fatal != nil {
		return u, fatal
	}
	if bad != nil {
		return u, errs.New(errs.ProviderError, "invalid model output after one repair: "+truncate(bad.Error(), 200), "")
	}
	return u, nil
}
