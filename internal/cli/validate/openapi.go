package validate

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
)

func (openAPIChecker) Check(ctx context.Context, _ string, data []byte) error {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return err
	}
	return doc.Validate(ctx)
}
