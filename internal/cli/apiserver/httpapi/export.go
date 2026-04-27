package httpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	apiclient "github.com/merionyx/api-gateway/internal/cli/apiserver"
	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
)

// ExportContracts calls POST /v1/contracts/export and returns file entries on success.
func ExportContracts(ctx context.Context, httpClient *http.Client, serverURL string, req apiclient.ExportRequest) ([]apiclient.ExportFile, error) {
	c, err := newClientWithResponses(serverURL, httpClient)
	if err != nil {
		return nil, err
	}
	resp, err := c.ExportContractsWithResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Files, nil
	}
	return nil, exportError(resp)
}

func exportError(resp *apiserverclient.ExportContractsResponse) error {
	if resp.ApplicationproblemJSON400 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON400))
	}
	if resp.ApplicationproblemJSON500 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON500))
	}
	if resp.ApplicationproblemJSON502 != nil {
		return fmt.Errorf("api: %s", problemString(resp.ApplicationproblemJSON502))
	}
	return fmt.Errorf("api: HTTP %d: %s", resp.StatusCode(), trimBody(resp.Body))
}

// DecodePayload decodes content_base64 from an export file entry.
func DecodePayload(f apiclient.ExportFile) ([]byte, error) {
	return base64.StdEncoding.DecodeString(f.ContentBase64)
}
