package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type Crypto struct{ key [32]byte }

func FromKey(key []byte) (*Crypto, error) {
	if len(key) != 32 {
		return nil, domain.NewError("master_key_missing", "expected 32 bytes")
	}
	c := &Crypto{}
	copy(c.key[:], key)
	return c, nil
}
func LoadOrCreate(path string, hasEncrypted bool) (*Crypto, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		if e = fileops.Secure(path); e != nil {
			return nil, e
		}
		return FromKey(b)
	}
	if !os.IsNotExist(e) {
		return nil, e
	}
	if hasEncrypted {
		return nil, domain.NewError("master_key_missing", "cannot decrypt stored secrets without master.key")
	}
	b = make([]byte, 32)
	if _, e = rand.Read(b); e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return nil, e
	}
	if e = fileops.Secure(path); e != nil {
		ce := f.Close()
		return nil, fmt.Errorf("secure key file: %w; close: %v", e, ce)
	}
	_, e = f.Write(b)
	syncErr := f.Sync()
	ce := f.Close()
	if syncErr != nil {
		return nil, syncErr
	}
	if e != nil {
		return nil, e
	}
	if ce != nil {
		return nil, ce
	}
	return FromKey(b)
}
func (c *Crypto) gcm() (cipher.AEAD, error) {
	b, e := aes.NewCipher(c.key[:])
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func (c *Crypto) Encrypt(plain string) (string, error) {
	g, e := c.gcm()
	if e != nil {
		return "", e
	}
	n := make([]byte, g.NonceSize())
	if _, e = rand.Read(n); e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(g.Seal(n, n, []byte(plain), nil)), nil
}
func (c *Crypto) Decrypt(encoded string) (string, error) {
	raw, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return "", domain.NewError("master_key_missing", "invalid ciphertext encoding")
	}
	g, e := c.gcm()
	if e != nil {
		return "", e
	}
	if len(raw) < g.NonceSize()+g.Overhead() {
		return "", domain.NewError("master_key_missing", "ciphertext too short")
	}
	p, e := g.Open(nil, raw[:12], raw[12:], nil)
	if e != nil {
		return "", domain.NewError("master_key_missing", "stored ciphertext cannot be decrypted")
	}
	if !utf8.Valid(p) {
		return "", domain.NewError("invalid_config", "secret is not UTF-8")
	}
	return string(p), nil
}
func KeyFingerprint(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func KeyPrefix(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return ""
	}
	if len(r) <= 8 {
		return string(r[:min(2, len(r))]) + "…"
	}
	return string(r[:4]) + "…" + string(r[len(r)-4:])
}
func Redact(s string) string { return KeyPrefix(s) }
