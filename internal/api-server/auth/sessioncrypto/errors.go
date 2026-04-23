package sessioncrypto

import "errors"

var (
	// ErrInvalidKEKSize means KEK material is not 32 bytes (AES-256).
	ErrInvalidKEKSize = errors.New("sessioncrypto: KEK must be 32 bytes (AES-256)")

	// ErrUnknownKEKID means the envelope's kek_id is not present in the keyring.
	ErrUnknownKEKID = errors.New("sessioncrypto: envelope kek_id not in keyring")

	// ErrDecrypt means ciphertext authentication failed or payload is malformed.
	ErrDecrypt = errors.New("sessioncrypto: decryption failed")

	// ErrUnsupportedEnvelopeVersion is returned for unknown envelope v.
	ErrUnsupportedEnvelopeVersion = errors.New("sessioncrypto: unsupported envelope version")

	// ErrDuplicateKEKID is returned from NewKeyring when two keys share the same ID.
	ErrDuplicateKEKID = errors.New("sessioncrypto: duplicate kek_id")
)
