package sessioncrypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
)

func testKEK(id, material string) KEK {
	h := sha256.Sum256([]byte(material))
	k := KEK{ID: id, Key: append([]byte(nil), h[:]...)}
	return k
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	active := testKEK("kek-2026-04-a", "alpha")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	want := []byte("idp-refresh-placeholder-not-real-secret")
	env, err := kr.Seal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Open(env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("plaintext mismatch")
	}
	ZeroBytes(got)
}

func TestRoundTripNilPlaintext(t *testing.T) {
	t.Parallel()
	active := testKEK("kek-a", "nil-plain")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	env, err := kr.Seal(nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Open(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got len %d", len(got))
	}
}

func TestJSONWireRoundTrip(t *testing.T) {
	t.Parallel()
	active := testKEK("wire-1", "json")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	env, err := kr.Seal([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("payload")) {
		t.Fatal("json must not contain plaintext")
	}

	env2, err := UnmarshalEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kr.Open(env2)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("got %q", got)
	}
}

func TestUnknownKEKID(t *testing.T) {
	t.Parallel()
	a := testKEK("a", "a")
	b := testKEK("b", "b")
	krA, err := NewKeyring(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { krA.Close() })

	env, err := krA.Seal([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}

	krB, err := NewKeyring(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { krB.Close() })

	_, err = krB.Open(env)
	if !errors.Is(err, ErrUnknownKEKID) {
		t.Fatalf("want ErrUnknownKEKID, got %v", err)
	}
}

func TestLegacyDecryptAfterRotation(t *testing.T) {
	t.Parallel()
	oldK := testKEK("old", "old-material")
	newK := testKEK("new", "new-material")

	krOld, err := NewKeyring(oldK)
	if err != nil {
		t.Fatal(err)
	}
	env, err := krOld.Seal([]byte("secret-session-field"))
	krOld.Close()
	if err != nil {
		t.Fatal(err)
	}

	krRotated, err := NewKeyring(newK, oldK)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { krRotated.Close() })

	got, err := krRotated.Open(env)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "secret-session-field" {
		t.Fatalf("got %q", got)
	}

	env2, err := krRotated.Seal([]byte("next"))
	if err != nil {
		t.Fatal(err)
	}
	if env2.KEKID != "new" {
		t.Fatalf("new seal should use active id new, got %q", env2.KEKID)
	}
}

func TestTamperCiphertext(t *testing.T) {
	t.Parallel()
	active := testKEK("k", "tamper")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	env, err := kr.Seal([]byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	env.Ciphertext[0] ^= 0xff
	_, err = kr.Open(env)
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
}

func TestInvalidKEKSize(t *testing.T) {
	t.Parallel()
	_, err := NewKeyring(KEK{ID: "x", Key: make([]byte, 16)})
	if !errors.Is(err, ErrInvalidKEKSize) {
		t.Fatalf("want ErrInvalidKEKSize, got %v", err)
	}
}

func TestDuplicateLegacyID(t *testing.T) {
	t.Parallel()
	a := testKEK("same", "x")
	_, err := NewKeyring(a, a)
	if !errors.Is(err, ErrDuplicateKEKID) {
		t.Fatalf("want ErrDuplicateKEKID, got %v", err)
	}
}

func TestZeroBytes(t *testing.T) {
	t.Parallel()
	b := []byte{1, 2, 3, 4}
	ZeroBytes(b)
	for _, v := range b {
		if v != 0 {
			t.Fatal("expected zeroed slice")
		}
	}
}

func TestEnvelopeBadJSON(t *testing.T) {
	t.Parallel()
	_, err := UnmarshalEnvelope([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnsupportedEnvelopeVersion(t *testing.T) {
	t.Parallel()
	active := testKEK("k", "v")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	env := Envelope{
		V:            999,
		Alg:          algV1,
		KEKID:        active.ID,
		WrapNonce:    make([]byte, gcmNonceSize),
		WrappedDEK:   make([]byte, gcmTagSize),
		DataNonce:    make([]byte, gcmNonceSize),
		Ciphertext:   make([]byte, gcmTagSize),
	}
	_, err = kr.Open(env)
	if !errors.Is(err, ErrUnsupportedEnvelopeVersion) {
		t.Fatalf("want ErrUnsupportedEnvelopeVersion, got %v", err)
	}
}

func TestJSONDoesNotEmitPlaintextInEnvelope(t *testing.T) {
	t.Parallel()
	secret := "super-secret-idp-refresh"
	active := testKEK("kid", "sek")
	kr, err := NewKeyring(active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	env, err := kr.Seal([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatal("envelope JSON leaks plaintext")
	}
}
