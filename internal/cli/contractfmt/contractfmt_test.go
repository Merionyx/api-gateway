package contractfmt

import (
	"strings"
	"testing"
)

func TestFormatBytes_XAGOnly_keyOrder(t *testing.T) {
	const in = `x-api-gateway:
  access:
    secure: false
  allowUndefinedMethods: false
  contract:
    name: proxy-list-05
  ownership:
    team: team-b
  prefix: /api/services/proxy-list-03/
  service:
    name: be-service-02
  version: v1
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "x-api-gateway:") {
		t.Fatalf("expected x-api-gateway as root key:\n%s", s)
	}
	if !strings.Contains(lines[1], "version:") {
		t.Fatalf("expected version as first field under x-api-gateway:\n%s", s)
	}
	if !strings.Contains(s, "ownership:") || !strings.Contains(s, "team: team-b") {
		t.Fatalf("expected unknown top-level keys under x-api-gateway to be preserved:\n%s", s)
	}
}

func TestFormatBytes_XAGOnly_nestedUnknownPreserved(t *testing.T) {
	const in = `x-api-gateway:
  version: v1
  access:
    secure: false
  contract:
    extra_field: kept
    name: proxy-list-05
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "extra_field: kept") {
		t.Fatalf("expected unknown keys under contract to be preserved:\n%s", s)
	}
}

func TestFormatBytes_OpenAPI_xapiGateway_extraTopLevelPreserved(t *testing.T) {
	const in = `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
x-api-gateway:
  version: v1
  prefix: /p/
  allowUndefinedMethods: false
  contract:
    name: c
  service:
    name: s
  access:
    secure: true
  tasesa: v1
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "tasesa: v1") {
		t.Fatalf("expected unknown top-level key tasesa preserved:\n%s", s)
	}
}

func TestFormatBytes_OpenAPI_operationFieldOrder(t *testing.T) {
	const in = `openapi: 3.0.0
info:
  title: t
  version: "1"
paths:
  /p:
    get:
      description: D-OP
      summary: S-OP
      responses:
        "200":
          description: OK
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	iSum := strings.Index(s, "summary: S-OP")
	iDesc := strings.Index(s, "description: D-OP")
	if iSum < 0 || iDesc < 0 {
		t.Fatalf("missing fields:\n%s", s)
	}
	if iSum > iDesc {
		t.Fatalf("want summary before operation description (OAS key order); got summary at %d, description at %d\n%s", iSum, iDesc, s)
	}
}

func TestFormatBytes_schemaPropertiesFieldOrder(t *testing.T) {
	// Regression: openapi3.Schemas (properties) must not fall through to yaml alphabetical key order.
	const in = `openapi: 3.0.0
info:
  title: t
  version: "1"
paths: {}
components:
  schemas:
    Root:
      type: object
      properties:
        data:
          description: inner
          title: InnerTitle
          type: string
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	iType := strings.Index(s, "type: string")
	iTitle := strings.Index(s, "title: InnerTitle")
	if iType < 0 || iTitle < 0 {
		t.Fatalf("missing nested schema fields:\n%s", s)
	}
	if iType > iTitle {
		t.Fatalf("want OAS key order (type before title) inside properties.data; got type at %d, title at %d\n%s", iType, iTitle, s)
	}
}

func TestFormatBytes_OpenAPI_rootKeyOrder(t *testing.T) {
	const in = `info:
  title: t
  version: "1"
openapi: 3.0.0
paths: {}
x-api-gateway:
  version: v1
  prefix: /p/
  allowUndefinedMethods: true
  contract:
    name: c
  service:
    name: s
  access:
    secure: false
`
	out, err := FormatBytes([]byte(in), ".yaml", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	idxOpen := strings.Index(s, "openapi:")
	idxInfo := strings.Index(s, "info:")
	idxPaths := strings.Index(s, "paths:")
	idxX := strings.Index(s, "x-api-gateway:")
	if idxOpen < 0 || idxInfo < 0 || idxPaths < 0 || idxX < 0 {
		t.Fatalf("missing keys:\n%s", s)
	}
	if !(idxOpen < idxInfo && idxInfo < idxPaths && idxPaths < idxX) {
		t.Fatalf("want openapi, info, paths, x-api-gateway in that order:\n%s", s)
	}
}
