package secrets

import "context"

// Provider stores and loads secret material without exposing backend details to
// higher layers.
type Provider interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
