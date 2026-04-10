// Package contractfmt — normalize Merionyx contracts (YAML/JSON): stable key order where we care,
// extensions / unknown keys pushed to the end of mappings.
//
// # OpenAPI 3.x
//
// I’m going through github.com/getkin/kin-openapi on purpose (see run.go + oas_encode.go): load into
// their types, then let their MarshalYAML / JSON do the work per Parameter, Schema, Operation, etc.
// Keeps us aligned with how kin models the spec.
//
// ## Lossy output — not a mystery bug
//
// kin often drops keys that are false / empty in Go, because in the model those fields are redundant
// with defaults. After fmt you’ll see stuff missing vs hand-written YAML, e.g.:
//
//   - allowEmptyValue: false, nullable: false, that kind of thing;
//   - optional fields that became zero in the struct;
//   - same idea under nested Schema (default, nullable, …) when the marshaler skips zero-ish values.
//
// So if someone complains “fmt ate my fields” — that’s this path, not random breakage in our code.
// Output is usually still valid OpenAPI; it’s just a thinner / more canonical file than the original.
//
// ## If I ever need byte-for-byte or “keep every explicit key”
//
// Then this kin-based encoder isn’t the right tool. Rough options I’d revisit (each has a cost):
//
//   1. “Preserve” mode: one parse to map[string]any or yaml.Node, only reorder keys, never emit via
//      *openapi3.T. Keeps explicit false and weird keys; I’d have to own ordering without kin’s
//      MarshalYAML; ref resolve on write wouldn’t happen unless I wire it separately.
//
//   2. yaml.Node only: shuffle MappingNode children, leave values alone — best for structure + comments,
//      more annoying to implement (need context: is this node an Operation or just a map).
//
//   3. Two modes in the CLI: current = canonical (kin), optional = preserve via (1) or (2). Means
//      two codepaths and more tests.
//
//   4. Fork kin to always print false: probably pointless — the struct still doesn’t remember “was
//      this false explicit in YAML”, so I wouldn’t bother unless something changes upstream.
//
// # x-api-gateway
//
// Here we keep the full map tree (XAGDoc, xag_encode.go): known slices define order, everything else
// at that level stays (sorted after the known keys). Nested junk under contract / service / access /
// apps isn’t run through the same lossy kin-style path for the whole block.
//
// # JSON out
//
// yamlToJSON in run.go: anchors, tags, weird YAML types — gone in the round-trip. I know.

package contractfmt
