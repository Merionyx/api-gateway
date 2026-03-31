package utils

import "strings"

func ExtractRepoRefContractFromKey(key []byte) (string, string, string) {
	parts := strings.Split(string(key), "/")
	repo, ref, contract := parts[3], parts[4], parts[6]
	ref = strings.ReplaceAll(ref, "%2F", "/")
	return repo, ref, contract
}
