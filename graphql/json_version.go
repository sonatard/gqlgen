package graphql

import (
	"context"
	"fmt"
)

// JSONVersion selects the JSON package gqlgen reads requests and writes
// responses with.
type JSONVersion int

const (
	// JSONv1 uses encoding/json. It is the default.
	JSONv1 JSONVersion = iota
	// JSONv2 uses encoding/json/v2 with its default behavior. It does not
	// reproduce encoding/json: numbers in variables arrive as
	// jsontext.Value rather than json.Number, HTML characters are not
	// escaped, map keys are written in no particular order, and duplicate
	// names and invalid UTF-8 are errors.
	JSONv2
)

func (v JSONVersion) String() string {
	switch v {
	case JSONv1:
		return "v1"
	case JSONv2:
		return "v2"
	default:
		return fmt.Sprintf("JSONVersion(%d)", int(v))
	}
}

// GetJSONVersion returns the JSON version of the operation in ctx, or JSONv1
// if ctx carries no operation.
func GetJSONVersion(ctx context.Context) JSONVersion {
	if !HasOperationContext(ctx) {
		return JSONv1
	}
	return GetOperationContext(ctx).JSONVersion
}
