package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileProvider is the encrypted-file fallback secret backend used when no
// platform-native store is available.
type FileProvider struct {
	mu   sync.Mutex
	path string
	key  []byte
}

func NewFileProvider(path string, key []byte) (*FileProvider, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("file secrets provider requires a 32-byte key")
	}
	return &FileProvider{
		path: path,
		key:  append([]byte(nil), key...),
	}, nil
}

func (p *FileProvider) Store(_ context.Context, key string, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	store, err := p.loadStoreLocked()
	if err != nil {
		return err
	}

	encrypted, err := p.encrypt(value)
	if err != nil {
		return err
	}
	store[key] = encrypted
	return p.saveStoreLocked(store)
}

func (p *FileProvider) Load(_ context.Context, key string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	store, err := p.loadStoreLocked()
	if err != nil {
		return nil, err
	}

	encrypted, ok := store[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return p.decrypt(encrypted)
}

func (p *FileProvider) Delete(_ context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	store, err := p.loadStoreLocked()
	if err != nil {
		return err
	}
	delete(store, key)
	return p.saveStoreLocked(store)
}

func (p *FileProvider) loadStoreLocked() (map[string]string, error) {
	store := map[string]string{}
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return store, nil
}

func (p *FileProvider) saveStoreLocked(store map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	tempPath := p.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, p.path)
}

func (p *FileProvider) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (p *FileProvider) decrypt(encoded string) ([]byte, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	data := ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, data, nil)
}

var _ Provider = (*FileProvider)(nil)
