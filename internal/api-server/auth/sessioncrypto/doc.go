// Package sessioncrypto implements envelope encryption for auth session material at rest (etcd):
// random DEK per Seal, payload encrypted with DEK (AES-256-GCM), DEK wrapped with KEK (AES-256-GCM).
//
// There is no HTTP surface here (roadmap step 8 — KEK/DEK without HTTP wiring).
//
// Key rotation: construct a Keyring with one active KEK for Seal and legacy KEKs for Open
// (e.g. two KEKs overlapping for 7 days per operator policy — see roadmap «Решения», п. 10).
package sessioncrypto
