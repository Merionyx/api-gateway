package sessioncrypto

// ZeroBytes overwrites b with zeros. Callers should use this on sensitive buffers
// (e.g. plaintext copies, DEK material) once values are no longer needed.
func ZeroBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	clear(b)
}
