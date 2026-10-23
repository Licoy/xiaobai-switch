package secure

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRustCompatibleAES256GCMVector(t *testing.T) {
	// NIST AES-256-GCM vector: zero key, zero 96-bit nonce, one zero block
	c, e := FromKey(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := hex.DecodeString("000000000000000000000000cea7403d4d606b6e074ec5d3baf39d18d0d1c8a799996bf0265b98b5d48ab919")
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.Decrypt(base64.StdEncoding.EncodeToString(raw))
	if e != nil {
		t.Fatal(e)
	}
	if p != string(make([]byte, 16)) {
		t.Fatalf("incorrect plaintext %x", p)
	}
	wrong, e := FromKey(make([]byte, 31))
	if e == nil || wrong != nil {
		t.Fatal("accepted short key")
	}
	wrong, e = FromKey([]byte("12345678901234567890123456789012"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = wrong.Decrypt(base64.StdEncoding.EncodeToString(raw)); e == nil {
		t.Fatal("accepted wrong key")
	}
	enc, e := c.Encrypt("sk-兼容-secret")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := c.Decrypt(enc); e != nil || got != "sk-兼容-secret" {
		t.Fatalf("round trip %q %v", got, e)
	}
}
func TestLoadMissingAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "master.key")
	if _, e := LoadOrCreate(path, true); e == nil {
		t.Fatal("created missing key for encrypted database")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("missing key written")
	}
	if _, e := LoadOrCreate(path, false); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil || len(b) != 32 {
		t.Fatalf("key size %d %v", len(b), e)
	}
	assertPrivateKey(t, path)
	if _, e = LoadOrCreate(path, true); e != nil {
		t.Fatal(e)
	}
	assertPrivateKey(t, path)
}
