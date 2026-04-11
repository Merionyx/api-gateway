package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

func jsonETag(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf(`"%x"`, sum), nil
}

func ifNoneMatchMatches(ifNoneMatch string, etag string) bool {
	in := strings.TrimSpace(ifNoneMatch)
	in = strings.TrimPrefix(in, "W/")
	in = strings.Trim(in, `"`)
	want := strings.Trim(etag, `"`)
	return in == want
}
