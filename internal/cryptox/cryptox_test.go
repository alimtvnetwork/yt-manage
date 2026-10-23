package cryptox_test

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/alimtvnetwork/yt-manage/internal/cryptox"
)

// fixedKey returns 32 deterministic bytes for tests.
func fixedKey() []byte {
	key := make([]byte, cryptox.KeySize)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

func TestLoadKeyFromEnvAndRoundTrip(t *testing.T) {
	want := fixedKey()
	t.Setenv("YT_MANAGE_MASTER_KEY", base64.StdEncoding.EncodeToString(want))

	key, err := cryptox.LoadKey(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	if !bytes.Equal(key, want) {
		t.Fatalf("LoadKey returned %v, want the env key bytes", key)
	}

	plaintext := []byte("sample-refresh-token-12345")
	ciphertext, nonce, err := cryptox.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext equals plaintext")
	}
	if len(nonce) == 0 {
		t.Error("nonce is empty")
	}
	got, err := cryptox.Decrypt(key, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt = %q, want %q", got, plaintext)
	}
}

func TestLoadKeyFromFile(t *testing.T) {
	t.Setenv("YT_MANAGE_MASTER_KEY", "")
	home := t.TempDir()

	key, err := cryptox.LoadKey(home)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}
	if len(key) != cryptox.KeySize {
		t.Fatalf("key length = %d, want %d", len(key), cryptox.KeySize)
	}

	info, err := os.Stat(filepath.Join(home, "master.key"))
	if err != nil {
		t.Fatalf("stat master.key: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("master.key perm = %o, want 600", perm)
	}

	again, err := cryptox.LoadKey(home)
	if err != nil {
		t.Fatalf("second LoadKey: %v", err)
	}
	if !bytes.Equal(again, key) {
		t.Error("second LoadKey returned different bytes")
	}

	plaintext := []byte("another-refresh-token")
	ciphertext, nonce, err := cryptox.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := cryptox.Decrypt(again, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("Decrypt = %q, want %q", got, plaintext)
	}
}

func TestLoadKeyFailures(t *testing.T) {
	t.Run("short env key", func(t *testing.T) {
		short := make([]byte, 16)
		t.Setenv("YT_MANAGE_MASTER_KEY", base64.StdEncoding.EncodeToString(short))
		if _, err := cryptox.LoadKey(t.TempDir()); err == nil {
			t.Error("LoadKey with 16-byte env key: expected error, got nil")
		}
	})
	t.Run("non base64 env key", func(t *testing.T) {
		t.Setenv("YT_MANAGE_MASTER_KEY", "%%%not-base64%%%")
		if _, err := cryptox.LoadKey(t.TempDir()); err == nil {
			t.Error("LoadKey with non-base64 env key: expected error, got nil")
		}
	})
}

func TestDecryptFailures(t *testing.T) {
	key := fixedKey()
	other := make([]byte, cryptox.KeySize)
	for i := range other {
		other[i] = byte(0xA0 + i)
	}
	plaintext := []byte("secret-token")
	ciphertext, nonce, err := cryptox.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	t.Run("wrong key", func(t *testing.T) {
		if _, err := cryptox.Decrypt(other, ciphertext, nonce); err == nil {
			t.Error("Decrypt with a different valid key: expected error, got nil")
		}
	})
	t.Run("truncated ciphertext", func(t *testing.T) {
		truncated := ciphertext[:len(ciphertext)/2]
		if _, err := cryptox.Decrypt(key, truncated, nonce); err == nil {
			t.Error("Decrypt with truncated ciphertext: expected error, got nil")
		}
	})
}
