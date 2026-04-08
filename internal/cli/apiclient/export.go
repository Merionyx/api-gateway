// Package apiclient calls API Server HTTP endpoints used by agwctl.
package apiclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ExportRequest is the JSON body for POST /api/v1/contracts/export.
type ExportRequest struct {
	Repository   string `json:"repository"`
	Ref          string `json:"ref"`
	Path         string `json:"path,omitempty"`
	ContractName string `json:"contract_name,omitempty"`
}

// ExportFile is one contract file in the export response.
type ExportFile struct {
	ContractName  string `json:"contract_name"`
	SourcePath    string `json:"source_path"`
	ContentBase64 string `json:"content_base64"`
}

type exportResponse struct {
	Files []ExportFile `json:"files"`
}

type apiErrorBody struct {
	Error string `json:"error"`
}

// ExportContracts POSTs to the API Server and returns decoded file entries.
func ExportContracts(ctx context.Context, serverURL string, req ExportRequest) ([]ExportFile, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	u := strings.TrimRight(serverURL, "/") + "/api/v1/contracts/export"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr apiErrorBody
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("api: %s", apiErr.Error)
		}
		return nil, fmt.Errorf("api: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed exportResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return parsed.Files, nil
}

// DecodePayload decodes content_base64 from an ExportFile.
func DecodePayload(f ExportFile) ([]byte, error) {
	return base64.StdEncoding.DecodeString(f.ContentBase64)
}
