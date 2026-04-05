package sftp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPoolClientReusesConnectionForSameEndpoint(t *testing.T) {
	var calls atomic.Int32
	pool := newPool(func(_ context.Context, opts Options) (*Client, error) {
		calls.Add(1)
		return &Client{opts: opts}, nil
	})

	clientA, err := pool.Client(context.Background(), Options{
		Host: "files.example.test",
		Port: 22,
		User: "demo",
		Path: "/alpha",
	})
	if err != nil {
		t.Fatalf("Client() error = %v", err)
	}

	clientB, err := pool.Client(context.Background(), Options{
		Host: "files.example.test",
		Port: 22,
		User: "demo",
		Path: "/beta",
	})
	if err != nil {
		t.Fatalf("Client() second call error = %v", err)
	}

	if clientA != clientB {
		t.Fatal("Client() returned different client instances for the same endpoint")
	}
	if calls.Load() != 1 {
		t.Fatalf("connector calls = %d, want 1", calls.Load())
	}
}

func TestPoolClientRetriesAfterConnectError(t *testing.T) {
	var calls atomic.Int32
	failOnce := true
	pool := newPool(func(_ context.Context, opts Options) (*Client, error) {
		calls.Add(1)
		if failOnce {
			failOnce = false
			return nil, errors.New("boom")
		}
		return &Client{opts: opts}, nil
	})

	_, err := pool.Client(context.Background(), Options{
		Host: "files.example.test",
		Port: 22,
		User: "demo",
		Path: "/",
	})
	if err == nil {
		t.Fatal("Client() error = nil, want first connect failure")
	}

	client, err := pool.Client(context.Background(), Options{
		Host: "files.example.test",
		Port: 22,
		User: "demo",
		Path: "/",
	})
	if err != nil {
		t.Fatalf("Client() retry error = %v", err)
	}
	if client == nil {
		t.Fatal("Client() retry returned nil client")
	}
	if calls.Load() != 2 {
		t.Fatalf("connector calls = %d, want 2 after retry", calls.Load())
	}
}

func TestPoolClientSharesInflightConnection(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	pool := newPool(func(_ context.Context, opts Options) (*Client, error) {
		calls.Add(1)
		close(started)
		<-release
		return &Client{opts: opts}, nil
	})

	type result struct {
		client *Client
		err    error
	}
	results := make(chan result, 2)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := pool.Client(context.Background(), Options{
				Host: "files.example.test",
				Port: 22,
				User: "demo",
				Path: "/shared",
			})
			results <- result{client: client, err: err}
		}()
	}

	<-started
	close(release)
	wg.Wait()
	close(results)

	var first *Client
	for result := range results {
		if result.err != nil {
			t.Fatalf("Client() concurrent error = %v", result.err)
		}
		if first == nil {
			first = result.client
			continue
		}
		if result.client != first {
			t.Fatal("concurrent Client() calls did not share the same client")
		}
	}

	if calls.Load() != 1 {
		t.Fatalf("connector calls = %d, want 1 for concurrent acquire", calls.Load())
	}
}

func TestPoolCloseRejectsFutureRequests(t *testing.T) {
	pool := newPool(func(_ context.Context, opts Options) (*Client, error) {
		return &Client{opts: opts}, nil
	})

	if err := pool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := pool.Client(context.Background(), Options{
		Host: "files.example.test",
		Port: 22,
		User: "demo",
		Path: "/",
	})
	if !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("Client() error = %v, want ErrPoolClosed", err)
	}
}
