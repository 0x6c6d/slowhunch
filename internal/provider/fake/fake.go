// Package fake provides an in-process Provider for tests.
package fake

import (
	"context"
	"errors"
	"sync"

	"github.com/0x6c6d/slowhunch/internal/provider"
)

// Fake is a scripted provider.
type Fake struct {
	Handler func(req provider.Request) (provider.Response, error)
	Replies map[string][]string
	Calls   []provider.Request

	mu sync.Mutex
}

// Complete records req and returns the handler's or the next scripted reply.
func (f *Fake) Complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, req)
	var resp provider.Response
	var err error
	if f.Handler != nil {
		f.mu.Unlock()
		resp, err = f.Handler(req)
		if err != nil {
			return resp, err
		}
	} else {
		q := f.Replies[req.Role]
		if len(q) == 0 {
			f.mu.Unlock()
			return provider.Response{}, errors.New("fake: no reply left for role " + req.Role)
		}
		resp.Text = q[0]
		f.Replies[req.Role] = q[1:]
		f.mu.Unlock()
	}
	if resp.TokensIn == 0 {
		resp.TokensIn = len(req.System+req.User) / 4
	}
	if resp.TokensOut == 0 {
		resp.TokensOut = len(resp.Text) / 4
	}
	return resp, nil
}

// CallsFor returns the recorded requests for role.
func (f *Fake) CallsFor(role string) []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []provider.Request
	for _, c := range f.Calls {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}
