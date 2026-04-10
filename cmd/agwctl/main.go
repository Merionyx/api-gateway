// agwctl — CLI for Merionyx API Gateway (contract export via REST).
//
// Example:
//
//	agwctl --server http://127.0.0.1:8080 ping
//	agwctl --server http://127.0.0.1:8080 contract export --repo my-repo --ref heads/main --out ./out
//
// Config (~/.config/agwctl/config.yaml), managed with `agwctl config`:
//
//	agwctl config set-context dev --server http://127.0.0.1:8080
//	agwctl config use-context dev
//
// YAML shape:
//
//	current-context: dev
//	contexts:
//	  dev:
//	    server: http://127.0.0.1:8080

package main

import "github.com/merionyx/api-gateway/internal/cli"

func main() {
	cli.Execute()
}
