// agwctl — CLI for Merionyx API Gateway (contract export via REST).
//
// Example:
//
//	agwctl --server http://127.0.0.1:8080 contract export --repo my-repo --ref heads/main --out ./out
//
// Config (~/.config/agwctl/config.yaml):
//
//	current-context: dev
//	contexts:
//	  dev:
//	    server: http://127.0.0.1:8080

package main

import "merionyx/api-gateway/internal/cli"

func main() {
	cli.Execute()
}
