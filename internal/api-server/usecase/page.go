package usecase

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

// DefaultListLimit matches components/parameters/Limit in the API Server OpenAPI spec.
const DefaultListLimit = 50

type offsetCursor struct {
	O int `json:"o"`
}

// DecodeOffsetCursor returns the start offset encoded in an opaque cursor (empty → 0).
func DecodeOffsetCursor(cursor *string) (int, error) {
	if cursor == nil || *cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(*cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	var oc offsetCursor
	if err := json.Unmarshal(raw, &oc); err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	if oc.O < 0 {
		return 0, fmt.Errorf("invalid cursor offset")
	}
	return oc.O, nil
}

// EncodeOffsetCursor builds the next-page cursor after consuming a page starting at offset.
func EncodeOffsetCursor(nextOffset int) string {
	b, _ := json.Marshal(offsetCursor{O: nextOffset})
	return base64.RawURLEncoding.EncodeToString(b)
}

// ResolveLimit applies the OpenAPI default when limit is nil or non-positive.
func ResolveLimit(limit *int) int {
	if limit == nil || *limit <= 0 {
		return DefaultListLimit
	}
	return *limit
}

// PageStringSlice returns a stable-sorted slice paginated by offset cursor.
func PageStringSlice(items []string, limit int, cursor *string) (page []string, nextCursor *string, hasMore bool, err error) {
	sort.Strings(items)
	off, err := DecodeOffsetCursor(cursor)
	if err != nil {
		return nil, nil, false, err
	}
	if off > len(items) {
		off = len(items)
	}
	end := off + limit
	if end > len(items) {
		end = len(items)
	}
	page = items[off:end]
	if end < len(items) {
		nc := EncodeOffsetCursor(end)
		nextCursor = &nc
		hasMore = true
	}
	return page, nextCursor, hasMore, nil
}

// PageSlice paginates an ordered slice using the same offset cursor encoding as PageStringSlice.
func PageSlice[T any](all []T, limit int, cursor *string) (page []T, nextCursor *string, hasMore bool, err error) {
	off, err := DecodeOffsetCursor(cursor)
	if err != nil {
		return nil, nil, false, err
	}
	if off > len(all) {
		off = len(all)
	}
	end := off + limit
	if end > len(all) {
		end = len(all)
	}
	page = all[off:end]
	if end < len(all) {
		nc := EncodeOffsetCursor(end)
		nextCursor = &nc
		hasMore = true
	}
	return page, nextCursor, hasMore, nil
}
