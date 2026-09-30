package crypto

import (
	"bytes"
	"errors"
	"testing"
)

var testParams = KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}

func TestSealOpenRoundTrip(t *testing.T) {
	key := RandomBytes(KeySize)
	for _, pt := range [][]byte{nil, []byte("x"), bytes.Repeat([]byte("secret "), 500)} {
		blob, err := Seal(key, pt, []byte("aad"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := Open(key, blob, []byte("aad"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("round trip mismatch for %d bytes", len(pt))
		}
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	key := RandomBytes(KeySize)
	a, _ := Seal(key, []byte("same"), nil)
	b, _ := Seal(key, []byte("same"), nil)
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext are identical")
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	key := RandomBytes(KeySize)
	blob, _ := Seal(key, []byte("secret"), []byte("aad"))

	for i := range blob {
		bad := bytes.Clone(blob)
		bad[i] ^= 0x01
		if _, err := Open(key, bad, []byte("aad")); !errors.Is(err, ErrOpen) {
			t.Fatalf("flipped bit at byte %d was accepted", i)
		}
	}
	if _, err := Open(key, blob, []byte("other")); !errors.Is(err, ErrOpen) {
		t.Fatal("wrong associated data was accepted")
	}
	if _, err := Open(RandomBytes(KeySize), blob, []byte("aad")); !errors.Is(err, ErrOpen) {
		t.Fatal("wrong key was accepted")
	}
	if _, err := Open(key, blob[:10], []byte("aad")); !errors.Is(err, ErrOpen) {
		t.Fatal("truncated blob was accepted")
	}
}

func TestDeriveKEK(t *testing.T) {
	salt := RandomBytes(SaltSize)
	a, err := DeriveKEK([]byte("correct horse"), salt, testParams)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Destroy()
	b, _ := DeriveKEK([]byte("correct horse"), salt, testParams)
	defer b.Destroy()
	c, _ := DeriveKEK([]byte("correct horsf"), salt, testParams)
	defer c.Destroy()
	d, _ := DeriveKEK([]byte("correct horse"), RandomBytes(SaltSize), testParams)
	defer d.Destroy()

	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("derivation is not deterministic")
	}
	if bytes.Equal(a.Bytes(), c.Bytes()) {
		t.Fatal("different passwords gave the same key")
	}
	if bytes.Equal(a.Bytes(), d.Bytes()) {
		t.Fatal("different salts gave the same key")
	}
	if a.Size() != KeySize {
		t.Fatalf("key size %d", a.Size())
	}
}

func TestDeriveKEKRejectsBadInput(t *testing.T) {
	salt := RandomBytes(SaltSize)
	bad := []KDFParams{
		{Time: 0, MemoryKiB: 8 * 1024, Threads: 1},
		{Time: 1, MemoryKiB: 16, Threads: 1},
		{Time: 1, MemoryKiB: 1 << 30, Threads: 1},
		{Time: 1, MemoryKiB: 8 * 1024, Threads: 0},
		{Time: 1000, MemoryKiB: 8 * 1024, Threads: 1},
	}
	for _, p := range bad {
		if _, err := DeriveKEK([]byte("pw"), salt, p); err == nil {
			t.Fatalf("params %+v were accepted", p)
		}
	}
	if _, err := DeriveKEK([]byte("pw"), salt[:4], testParams); err == nil {
		t.Fatal("short salt was accepted")
	}
	if _, err := DeriveKEK(nil, salt, testParams); err == nil {
		t.Fatal("empty password was accepted")
	}
	if err := DefaultKDFParams().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeriveSubkeyIsDomainSeparated(t *testing.T) {
	mk := NewMasterKey()
	defer mk.Destroy()
	a, err := DeriveSubkey(mk.Bytes(), InfoDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Destroy()
	b, _ := DeriveSubkey(mk.Bytes(), InfoFields)
	defer b.Destroy()
	a2, _ := DeriveSubkey(mk.Bytes(), InfoDatabase)
	defer a2.Destroy()

	if bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("sub-keys for different purposes are equal")
	}
	if !bytes.Equal(a.Bytes(), a2.Bytes()) {
		t.Fatal("sub-key derivation is not deterministic")
	}
	if bytes.Equal(a.Bytes(), mk.Bytes()) {
		t.Fatal("sub-key equals master key")
	}
}

func TestWipe(t *testing.T) {
	b := []byte("secret")
	Wipe(b)
	if !bytes.Equal(b, make([]byte, 6)) {
		t.Fatal("not wiped")
	}
}
