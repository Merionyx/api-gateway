package sessioncrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	gcmNonceSize = 12
	gcmTagSize   = 16 // appended by cipher.NewGCM Seal
	dekSize      = 32
	envelopeV1   = 1
	algV1        = "AES-256-GCM-envelope-v1"
)

// Envelope is a versioned JSON blob suitable for embedding in an etcd session value (ш. 9).
type Envelope struct {
	V          int    `json:"v"`
	Alg        string `json:"alg"`
	KEKID      string `json:"kek_id"`
	WrapNonce  []byte `json:"wrap_nonce"`
	WrappedDEK []byte `json:"wrapped_dek"`
	DataNonce  []byte `json:"data_nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// Seal encrypts plaintext with a fresh random DEK and wraps that DEK with the active KEK.
// plaintext may be nil (treated as empty). Caller must not log returned fields.
func (k *Keyring) Seal(plaintext []byte) (Envelope, error) {
	if k == nil || len(k.active) != kekSize {
		return Envelope{}, errors.New("sessioncrypto: nil or closed keyring")
	}
	if plaintext == nil {
		plaintext = []byte{}
	}

	dek := make([]byte, dekSize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return Envelope{}, fmt.Errorf("sessioncrypto: rand dek: %w", err)
	}
	defer ZeroBytes(dek)

	ptCipher, err := aes.NewCipher(dek)
	if err != nil {
		return Envelope{}, err
	}
	ptGCM, err := cipher.NewGCM(ptCipher)
	if err != nil {
		return Envelope{}, err
	}
	dataNonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, dataNonce); err != nil {
		ZeroBytes(dataNonce)
		return Envelope{}, fmt.Errorf("sessioncrypto: rand data nonce: %w", err)
	}
	ciphertext := ptGCM.Seal(nil, dataNonce, plaintext, nil)

	kCipher, err := aes.NewCipher(k.active)
	if err != nil {
		return Envelope{}, err
	}
	kGCM, err := cipher.NewGCM(kCipher)
	if err != nil {
		return Envelope{}, err
	}
	wrapNonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, wrapNonce); err != nil {
		ZeroBytes(wrapNonce)
		return Envelope{}, fmt.Errorf("sessioncrypto: rand wrap nonce: %w", err)
	}
	wrappedDEK := kGCM.Seal(nil, wrapNonce, dek, nil)

	return Envelope{
		V:          envelopeV1,
		Alg:        algV1,
		KEKID:      k.activeID,
		WrapNonce:  wrapNonce,
		WrappedDEK: wrappedDEK,
		DataNonce:  dataNonce,
		Ciphertext: ciphertext,
	}, nil
}

// Open decrypts an envelope. Returned plaintext is a copy independent of keyring buffers.
func (k *Keyring) Open(env Envelope) ([]byte, error) {
	if k == nil {
		return nil, errors.New("sessioncrypto: nil keyring")
	}
	if env.V != envelopeV1 || env.Alg != algV1 {
		return nil, ErrUnsupportedEnvelopeVersion
	}
	if env.KEKID == "" {
		return nil, ErrDecrypt
	}
	kek, ok := k.byID[env.KEKID]
	if !ok || len(kek) != kekSize {
		return nil, ErrUnknownKEKID
	}
	if len(env.WrapNonce) != gcmNonceSize || len(env.DataNonce) != gcmNonceSize {
		return nil, ErrDecrypt
	}
	if len(env.WrappedDEK) < gcmTagSize || len(env.Ciphertext) < gcmTagSize {
		return nil, ErrDecrypt
	}

	kCipher, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	kGCM, err := cipher.NewGCM(kCipher)
	if err != nil {
		return nil, err
	}
	dekPlain, err := kGCM.Open(nil, env.WrapNonce, env.WrappedDEK, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	defer ZeroBytes(dekPlain)
	if len(dekPlain) != dekSize {
		return nil, ErrDecrypt
	}

	ptCipher, err := aes.NewCipher(dekPlain)
	if err != nil {
		return nil, err
	}
	ptGCM, err := cipher.NewGCM(ptCipher)
	if err != nil {
		return nil, err
	}
	out, err := ptGCM.Open(nil, env.DataNonce, env.Ciphertext, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return append([]byte(nil), out...), nil
}

// MarshalEnvelope returns JSON suitable for etcd. Caller may zero plaintext after this if needed.
func MarshalEnvelope(e Envelope) ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEnvelope parses JSON produced by MarshalEnvelope.
func UnmarshalEnvelope(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("sessioncrypto: envelope json: %w", err)
	}
	return e, nil
}
