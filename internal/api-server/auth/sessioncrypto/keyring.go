package sessioncrypto

import (
	"fmt"
)

const kekSize = 32 // AES-256

// KEK identifies a 32-byte AES-256 key encryption key (operator-loaded, e.g. from K8s Secret).
type KEK struct {
	ID  string
	Key []byte
}

// Keyring holds the active KEK for new envelopes plus optional legacy KEKs for Open.
type Keyring struct {
	activeID string
	active   []byte
	byID     map[string][]byte // id -> owned 32-byte copy
}

// NewKeyring returns a keyring. active is used for Seal; legacy must list older KEKs still
// needed to decrypt existing envelopes (overlap rotation). active must not appear twice.
func NewKeyring(active KEK, legacy ...KEK) (*Keyring, error) {
	if active.ID == "" {
		return nil, fmt.Errorf("sessioncrypto: active KEK id is empty")
	}
	ak, err := cloneKEKKey(active.Key)
	if err != nil {
		return nil, err
	}
	byID := make(map[string][]byte, 1+len(legacy))
	byID[active.ID] = ak

	for _, lk := range legacy {
		if lk.ID == "" {
			ZeroBytes(ak)
			for _, v := range byID {
				ZeroBytes(v)
			}
			return nil, fmt.Errorf("sessioncrypto: legacy KEK id is empty")
		}
		if _, dup := byID[lk.ID]; dup {
			for _, v := range byID {
				ZeroBytes(v)
			}
			return nil, fmt.Errorf("%w: %q", ErrDuplicateKEKID, lk.ID)
		}
		k, err := cloneKEKKey(lk.Key)
		if err != nil {
			for _, v := range byID {
				ZeroBytes(v)
			}
			return nil, err
		}
		byID[lk.ID] = k
	}

	return &Keyring{
		activeID: active.ID,
		active:   ak,
		byID:     byID,
	}, nil
}

func cloneKEKKey(key []byte) ([]byte, error) {
	if len(key) != kekSize {
		return nil, ErrInvalidKEKSize
	}
	out := make([]byte, kekSize)
	copy(out, key)
	return out, nil
}

// Close zeros key material held by the keyring. Do not use the keyring after Close.
func (k *Keyring) Close() {
	if k == nil {
		return
	}
	ZeroBytes(k.active)
	for id := range k.byID {
		ZeroBytes(k.byID[id])
		delete(k.byID, id)
	}
	k.active = nil
	k.byID = nil
}
