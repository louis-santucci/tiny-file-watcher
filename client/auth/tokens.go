package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"tiny-file-watcher/internal"
)

// TokenSet holds the tokens persisted to disk.
type TokenSet struct {
	IDToken      string    `json:"id_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}

var tokensPath = internal.TokensPath

// SaveTokens writes ts to ~/.tfw/tokens.json with mode 0600.
func SaveTokens(ts TokenSet) error {
	path := tokensPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.Marshal(ts)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadTokens reads the stored TokenSet from disk.
// Returns os.ErrNotExist (wrapped) if the file does not exist.
func LoadTokens() (TokenSet, error) {
	data, err := os.ReadFile(tokensPath())
	if err != nil {
		return TokenSet{}, fmt.Errorf("read tokens: %w", err)
	}
	var ts TokenSet
	if err := json.Unmarshal(data, &ts); err != nil {
		return TokenSet{}, fmt.Errorf("parse tokens: %w", err)
	}
	return ts, nil
}

// ClearTokens removes the stored tokens file.
func ClearTokens() error {
	err := os.Remove(tokensPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
