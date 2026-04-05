package sftp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var ErrPoolClosed = errors.New("sftp connection pool is closed")

type connectFunc func(context.Context, Options) (*Client, error)

// Pool caches SSH/SFTP clients by connection target so higher layers can reuse
// transports across multiple filesystem operations.
type Pool struct {
	mu        sync.Mutex
	closed    bool
	connector connectFunc
	entries   map[string]*poolEntry
}

type poolEntry struct {
	ready  chan struct{}
	client *Client
	err    error
}

// NewPool builds a connection pool that creates clients with Connect.
func NewPool() *Pool {
	return newPool(Connect)
}

func newPool(connector connectFunc) *Pool {
	if connector == nil {
		connector = Connect
	}
	return &Pool{
		connector: connector,
		entries:   make(map[string]*poolEntry),
	}
}

// Client returns a shared client for the given connection target. Calls that
// differ only by remote path reuse the same SSH/SFTP transport.
func (p *Pool) Client(ctx context.Context, opts Options) (*Client, error) {
	if p == nil {
		return nil, fmt.Errorf("sftp pool is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}

	key := connectionKey(normalized)
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}

		if entry, ok := p.entries[key]; ok {
			p.mu.Unlock()
			select {
			case <-entry.ready:
				if entry.err != nil {
					return nil, entry.err
				}
				return entry.client, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		entry := &poolEntry{ready: make(chan struct{})}
		p.entries[key] = entry
		p.mu.Unlock()

		client, err := p.connector(ctx, normalized)

		p.mu.Lock()
		if p.closed {
			delete(p.entries, key)
			entry.err = ErrPoolClosed
			close(entry.ready)
			p.mu.Unlock()
			if client != nil {
				_ = client.Close()
			}
			return nil, ErrPoolClosed
		}

		if err != nil {
			delete(p.entries, key)
			entry.err = err
			close(entry.ready)
			p.mu.Unlock()
			return nil, err
		}

		entry.client = client
		close(entry.ready)
		p.mu.Unlock()
		return client, nil
	}
}

// Close closes every cached client and prevents future acquisitions.
func (p *Pool) Close() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true

	entries := make([]*poolEntry, 0, len(p.entries))
	for _, entry := range p.entries {
		entries = append(entries, entry)
	}
	p.entries = make(map[string]*poolEntry)
	p.mu.Unlock()

	var err error
	for _, entry := range entries {
		select {
		case <-entry.ready:
			if entry.client != nil {
				err = errors.Join(err, entry.client.Close())
			}
		default:
		}
	}
	return err
}

func connectionKey(opts Options) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(opts.Host)),
		fmt.Sprintf("%d", opts.Port),
		strings.TrimSpace(opts.User),
		strings.TrimSpace(opts.Auth),
		bestEffortExpand(strings.TrimSpace(opts.IdentityFile)),
		bestEffortExpand(strings.TrimSpace(opts.KnownHostsFile)),
	}, "\x00")
}

func bestEffortExpand(raw string) string {
	if raw == "" {
		return ""
	}
	expanded, err := expandPath(raw)
	if err != nil {
		return raw
	}
	return expanded
}
