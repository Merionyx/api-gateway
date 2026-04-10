package contractfmt

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"

	"gopkg.in/yaml.v3"
)

// OpenAPI (OAS) YAML emission: key order aligned with kin-openapi v0.135 MarshalYAML per type (unknown keys last, sorted).

var (
	oasOperationKeys = []string{
		"tags", "summary", "description", "operationId", "parameters", "requestBody",
		"responses", "callbacks", "deprecated", "security", "servers", "externalDocs",
	}
	oasPathItemKeys = []string{
		"summary", "description",
		"get", "post", "put", "patch", "delete", "connect", "head", "options", "trace",
		"servers", "parameters",
	}
	oasInfoKeys      = []string{"title", "description", "termsOfService", "contact", "license", "version"}
	oasResponseKeys  = []string{"description", "headers", "content", "links"}
	oasMediaTypeKeys = []string{"schema", "example", "examples", "encoding"}
	oasParameterKeys = []string{
		"name", "in", "description", "style", "explode", "allowEmptyValue", "allowReserved",
		"deprecated", "required", "schema", "example", "examples", "content",
	}
	oasRequestBodyKeys = []string{"description", "required", "content"}
	oasComponentsKeys  = []string{
		"schemas", "parameters", "headers", "requestBodies", "responses",
		"securitySchemes", "examples", "links", "callbacks",
	}
	oasServerKeys         = []string{"url", "description", "variables"}
	oasTagKeys            = []string{"name", "description", "externalDocs"}
	oasExternalDocsKeys   = []string{"description", "url"}
	oasRefKeys            = []string{"$ref"}
	oasServerVariableKeys = []string{"enum", "default", "description"}
	// Schema: kin-openapi openapi3/schema.go MarshalYAML insertion order after extensions.
	oasSchemaKeys = []string{
		"oneOf", "anyOf", "allOf", "not", "type", "title", "format", "description", "enum",
		"default", "example", "externalDocs",
		"uniqueItems", "exclusiveMinimum", "exclusiveMaximum",
		"nullable", "readOnly", "writeOnly", "allowEmptyValue", "deprecated", "xml",
		"minimum", "maximum", "multipleOf",
		"minLength", "maxLength", "pattern",
		"minItems", "maxItems", "items",
		"required", "properties", "minProperties", "maxProperties", "additionalProperties", "discriminator",
	}
)

func encodeYAMLNode(v any) (*yaml.Node, error) {
	return encodeValueOrdered(v)
}

func encodeNullNode() *yaml.Node {
	var n yaml.Node
	n.Kind = yaml.ScalarNode
	n.Tag = "!!null"
	return &n
}

// orderedKinMap emits a YAML mapping: stdKeys in list order first, then unknown keys sorted last.
// Shared by OAS and XAG formatters (each passes its own key slice).
func orderedKinMap(m map[string]any, stdKeys []string) (*yaml.Node, error) {
	if m == nil {
		return encodeNullNode(), nil
	}
	if len(stdKeys) == 0 {
		return buildMappingNode(m, sortedStringKeys(m))
	}
	stdSet := make(map[string]struct{}, len(stdKeys))
	for _, k := range stdKeys {
		stdSet[k] = struct{}{}
	}
	var unknown []string
	for k := range m {
		if _, ok := stdSet[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	order := append(pickPresentStdKeys(m, stdKeys), unknown...)
	return buildMappingNode(m, order)
}

func pickPresentStdKeys(m map[string]any, stdKeys []string) []string {
	var out []string
	for _, k := range stdKeys {
		if _, ok := m[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// looksLikeJSONSchema detects plain map[string]any blobs that are OpenAPI/JSON Schema objects
// (e.g. nested fragments) so we apply oasSchemaKeys instead of alphabetical order.
// WARN: Any mapping with a top-level "type" key is treated as schema — unrelated objects that only share that key get schema key order.
func looksLikeJSONSchema(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	if _, ok := m["$ref"]; ok {
		return true
	}
	for _, k := range []string{
		"properties", "items", "additionalProperties",
		"allOf", "oneOf", "anyOf", "not",
		"discriminator", "enum",
	} {
		if _, ok := m[k]; ok {
			return true
		}
	}
	if _, ok := m["type"]; ok {
		return true
	}
	return false
}

func orderedMapOrJSONSchema(m map[string]any) (*yaml.Node, error) {
	if looksLikeJSONSchema(m) {
		return orderedKinMap(m, oasSchemaKeys)
	}
	return orderedKinMap(m, nil)
}

// encodeSchemasMap encodes openapi3.Schemas (properties map): sorted property names, values via encodeValueOrdered.
func encodeSchemasMap(s openapi3.Schemas) (*yaml.Node, error) {
	if len(s) == 0 {
		var n yaml.Node
		n.Kind = yaml.MappingNode
		return &n, nil
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m := make(map[string]any, len(s))
	for _, k := range keys {
		m[k] = s[k]
	}
	return buildMappingNode(m, keys)
}

func buildMappingNode(m map[string]any, keyOrder []string) (*yaml.Node, error) {
	var mapping yaml.Node
	mapping.Kind = yaml.MappingNode
	for _, k := range keyOrder {
		val, ok := m[k]
		if !ok {
			continue
		}
		var kn yaml.Node
		if err := kn.Encode(k); err != nil {
			return nil, err
		}
		vn, err := encodeValueOrdered(val)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", k, err)
		}
		mapping.Content = append(mapping.Content, &kn, vn)
	}
	return &mapping, nil
}

func encodeOASMarshaler(m interface{ MarshalYAML() (any, error) }, stdKeys []string) (*yaml.Node, error) {
	raw, err := m.MarshalYAML()
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return encodeNullNode(), nil
	}
	mm, ok := raw.(map[string]any)
	if !ok {
		return encodeValueOrdered(raw)
	}
	return orderedKinMap(mm, stdKeys)
}

func encodePathItem(pi *openapi3.PathItem) (*yaml.Node, error) {
	if pi == nil {
		return encodeNullNode(), nil
	}
	raw, err := pi.MarshalYAML()
	if err != nil {
		return nil, err
	}
	switch x := raw.(type) {
	case map[string]any:
		return orderedKinMap(x, oasPathItemKeys)
	case openapi3.Ref:
		return encodeOASMarshaler(&x, oasRefKeys)
	default:
		return encodeValueOrdered(raw)
	}
}

func encodePaths(paths *openapi3.Paths) (*yaml.Node, error) {
	if paths == nil {
		return encodeNullNode(), nil
	}
	m := make(map[string]any)
	if paths.Extensions != nil {
		for k, v := range paths.Extensions {
			m[k] = v
		}
	}
	for k, v := range paths.Map() {
		m[k] = v
	}
	order := mergedPathOrder(paths)
	return buildMappingNode(m, order)
}

func mergedPathOrder(paths *openapi3.Paths) []string {
	var ext []string
	if paths.Extensions != nil {
		for k := range paths.Extensions {
			ext = append(ext, k)
		}
		sort.Strings(ext)
	}
	var pathKeys []string
	for k := range paths.Map() {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)
	return append(pathKeys, ext...)
}

func encodeResponses(resp *openapi3.Responses) (*yaml.Node, error) {
	if resp == nil {
		return encodeNullNode(), nil
	}
	var ext []string
	if resp.Extensions != nil {
		for k := range resp.Extensions {
			ext = append(ext, k)
		}
		sort.Strings(ext)
	}
	var codes []string
	for k := range resp.Map() {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	order := append(codes, ext...)

	m := make(map[string]any, len(order))
	if resp.Extensions != nil {
		for k, v := range resp.Extensions {
			m[k] = v
		}
	}
	for _, code := range codes {
		m[code] = resp.Value(code)
	}
	return buildMappingNode(m, order)
}

func encodeContent(c openapi3.Content) (*yaml.Node, error) {
	if len(c) == 0 {
		var n yaml.Node
		n.Kind = yaml.MappingNode
		return &n, nil
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m := make(map[string]any, len(c))
	for k, v := range c {
		m[k] = v
	}
	return buildMappingNode(m, keys)
}

func encodeParameters(p openapi3.Parameters) (*yaml.Node, error) {
	var seq yaml.Node
	seq.Kind = yaml.SequenceNode
	for _, ref := range p {
		vn, err := encodeValueOrdered(ref)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, vn)
	}
	return &seq, nil
}

func encodeServers(servers openapi3.Servers) (*yaml.Node, error) {
	var seq yaml.Node
	seq.Kind = yaml.SequenceNode
	for _, s := range servers {
		vn, err := encodeValueOrdered(s)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, vn)
	}
	return &seq, nil
}

func encodeTags(tags openapi3.Tags) (*yaml.Node, error) {
	var seq yaml.Node
	seq.Kind = yaml.SequenceNode
	for _, t := range tags {
		vn, err := encodeValueOrdered(t)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, vn)
	}
	return &seq, nil
}

func encodeSecurityRequirements(srs openapi3.SecurityRequirements) (*yaml.Node, error) {
	var seq yaml.Node
	seq.Kind = yaml.SequenceNode
	for _, sr := range srs {
		vn, err := encodeSecurityRequirement(sr)
		if err != nil {
			return nil, err
		}
		seq.Content = append(seq.Content, vn)
	}
	return &seq, nil
}

func encodeSecurityRequirement(sr openapi3.SecurityRequirement) (*yaml.Node, error) {
	if sr == nil {
		return encodeNullNode(), nil
	}
	m := make(map[string]any, len(sr))
	for k, v := range sr {
		m[k] = v
	}
	return buildMappingNode(m, sortedStringKeys(m))
}

func encodeValueOrdered(v any) (*yaml.Node, error) {
	if v == nil {
		return encodeNullNode(), nil
	}

	switch x := v.(type) {
	case map[string]any:
		return orderedMapOrJSONSchema(x)
	case openapi3.Schemas:
		return encodeSchemasMap(x)
	case *openapi3.Operation:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(x, oasOperationKeys)
	case openapi3.Operation:
		return encodeOASMarshaler(&x, oasOperationKeys)
	case *openapi3.PathItem:
		return encodePathItem(x)
	case openapi3.PathItem:
		return encodePathItem(&x)
	case *openapi3.Paths:
		return encodePaths(x)
	case *openapi3.Responses:
		return encodeResponses(x)
	case *openapi3.Response:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(x, oasResponseKeys)
	case openapi3.Response:
		return encodeOASMarshaler(x, oasResponseKeys)
	case *openapi3.Info:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(x, oasInfoKeys)
	case *openapi3.Components:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(x, oasComponentsKeys)
	case openapi3.Components:
		return encodeOASMarshaler(x, oasComponentsKeys)
	case *openapi3.Schema:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasSchemaKeys)
	case openapi3.Schema:
		return encodeOASMarshaler(x, oasSchemaKeys)
	case *openapi3.MediaType:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasMediaTypeKeys)
	case openapi3.MediaType:
		return encodeOASMarshaler(x, oasMediaTypeKeys)
	case openapi3.Content:
		return encodeContent(x)
	case *openapi3.Parameter:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasParameterKeys)
	case openapi3.Parameter:
		return encodeOASMarshaler(x, oasParameterKeys)
	case *openapi3.Header:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(x.Parameter, oasParameterKeys)
	case openapi3.Header:
		return encodeOASMarshaler(x.Parameter, oasParameterKeys)
	case *openapi3.RequestBody:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasRequestBodyKeys)
	case openapi3.RequestBody:
		return encodeOASMarshaler(x, oasRequestBodyKeys)
	case *openapi3.Server:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasServerKeys)
	case openapi3.Server:
		return encodeOASMarshaler(x, oasServerKeys)
	case openapi3.Servers:
		return encodeServers(x)
	case *openapi3.Tag:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasTagKeys)
	case openapi3.Tag:
		return encodeOASMarshaler(x, oasTagKeys)
	case openapi3.Tags:
		return encodeTags(x)
	case *openapi3.ExternalDocs:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasExternalDocsKeys)
	case openapi3.ExternalDocs:
		return encodeOASMarshaler(x, oasExternalDocsKeys)
	case openapi3.Ref:
		return encodeOASMarshaler(x, oasRefKeys)
	case *openapi3.Ref:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasRefKeys)
	case *openapi3.SchemaRef:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeSchemaRef(x)
	case openapi3.SchemaRef:
		return encodeSchemaRef(&x)
	case *openapi3.ParameterRef:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeParameterRef(x)
	case openapi3.ParameterRef:
		return encodeParameterRef(&x)
	case *openapi3.ResponseRef:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeResponseRef(x)
	case openapi3.ResponseRef:
		return encodeResponseRef(&x)
	case *openapi3.RequestBodyRef:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeRequestBodyRef(x)
	case openapi3.RequestBodyRef:
		return encodeRequestBodyRef(&x)
	case *openapi3.HeaderRef:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeHeaderRef(x)
	case openapi3.HeaderRef:
		return encodeHeaderRef(&x)
	case openapi3.Parameters:
		return encodeParameters(x)
	case openapi3.SecurityRequirements:
		return encodeSecurityRequirements(x)
	case openapi3.ServerVariable:
		return encodeOASMarshaler(x, oasServerVariableKeys)
	case *openapi3.ServerVariable:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeOASMarshaler(*x, oasServerVariableKeys)
	case XAGDoc:
		return encodeXAGDoc(x)
	case *XAGDoc:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeXAGDoc(*x)
	case xagContractMap:
		return encodeXAGContractMap(x)
	case xagServiceMap:
		return encodeXAGServiceMap(x)
	case xagAccessMap:
		return encodeXAGAccessMap(x)
	case xagAppMap:
		return encodeXAGAppMap(x)
	case *XAGV1:
		return encodeXAGV1(x)
	case XAGV1:
		return encodeXAGV1(&x)
	case *XAGContract:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeXAGContract(*x)
	case XAGContract:
		return encodeXAGContract(x)
	case *XAGService:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeXAGService(*x)
	case XAGService:
		return encodeXAGService(x)
	case *XAGAccess:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeXAGAccess(*x)
	case XAGAccess:
		return encodeXAGAccess(x)
	case *XAGApp:
		if x == nil {
			return encodeNullNode(), nil
		}
		return encodeXAGApp(*x)
	case XAGApp:
		return encodeXAGApp(x)
	case []XAGApp:
		return encodeXAGAppSlice(x)
	}

	// Dereference one level of pointer for known OpenAPI types.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		if m, ok := rv.Interface().(interface{ MarshalYAML() (any, error) }); ok {
			raw, err := m.MarshalYAML()
			if err != nil {
				return nil, err
			}
			if raw == nil {
				return encodeNullNode(), nil
			}
			if mm, ok := raw.(map[string]any); ok {
				return orderedMapOrJSONSchema(mm)
			}
			return encodeValueOrdered(raw)
		}
	}

	// TODO(contractfmt): Unhandled types fall back to yaml.Encode — nested map key order may not match OAS/XAG conventions; add explicit cases when new kin-openapi shapes appear in contracts.
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return &n, nil
}

func encodeSchemaRef(x *openapi3.SchemaRef) (*yaml.Node, error) {
	if x == nil {
		return encodeNullNode(), nil
	}
	if x.Ref != "" {
		return encodeValueOrdered(&openapi3.Ref{Ref: x.Ref, Extensions: x.Extensions})
	}
	if x.Value != nil {
		return encodeValueOrdered(x.Value)
	}
	return encodeNullNode(), nil
}

func encodeParameterRef(x *openapi3.ParameterRef) (*yaml.Node, error) {
	raw, err := x.MarshalYAML()
	if err != nil {
		return nil, err
	}
	switch t := raw.(type) {
	case map[string]any:
		return orderedKinMap(t, oasParameterKeys)
	case *openapi3.Ref:
		return encodeValueOrdered(t)
	default:
		return encodeValueOrdered(raw)
	}
}

func encodeResponseRef(x *openapi3.ResponseRef) (*yaml.Node, error) {
	raw, err := x.MarshalYAML()
	if err != nil {
		return nil, err
	}
	switch t := raw.(type) {
	case map[string]any:
		return orderedKinMap(t, oasResponseKeys)
	case *openapi3.Ref:
		return encodeValueOrdered(t)
	default:
		return encodeValueOrdered(raw)
	}
}

func encodeRequestBodyRef(x *openapi3.RequestBodyRef) (*yaml.Node, error) {
	raw, err := x.MarshalYAML()
	if err != nil {
		return nil, err
	}
	switch t := raw.(type) {
	case map[string]any:
		return orderedKinMap(t, oasRequestBodyKeys)
	case *openapi3.Ref:
		return encodeValueOrdered(t)
	default:
		return encodeValueOrdered(raw)
	}
}

func encodeHeaderRef(x *openapi3.HeaderRef) (*yaml.Node, error) {
	if x == nil {
		return encodeNullNode(), nil
	}
	if x.Ref != "" {
		return encodeValueOrdered(&openapi3.Ref{Ref: x.Ref, Extensions: x.Extensions})
	}
	if x.Value != nil {
		return encodeValueOrdered(x.Value)
	}
	return encodeNullNode(), nil
}
