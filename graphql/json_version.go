package graphql

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"io"
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

// JSONMode is the JSON mode an operation runs in. The executor shares one
// JSONMode with all of its operations, so it must not be modified while the
// executor serves requests.
type JSONMode struct {
	// Version selects encoding/json or encoding/json/v2.
	Version JSONVersion
	// ResponseOptions are added to the encoding/json/v2 options responses and
	// the built-in Any and Map scalars are written with in the JSONv2 mode.
	ResponseOptions json.Options
}

// GetJSONMode returns the JSON mode of the operation in ctx, or nil if ctx
// carries no operation or the operation has no mode, which both mean JSONv1.
func GetJSONMode(ctx context.Context) *JSONMode {
	if !HasOperationContext(ctx) {
		return nil
	}
	return GetOperationContext(ctx).JSONMode
}

// GetJSONVersion returns the JSON version of the operation in ctx, or JSONv1
// if ctx carries no operation.
func GetJSONVersion(ctx context.Context) JSONVersion {
	if m := GetJSONMode(ctx); m != nil {
		return m.Version
	}
	return JSONv1
}

// writeJSONContext writes v with the JSON package the operation in ctx
// selects. In the JSONv1 mode it writes what MarshalAny and MarshalMap write,
// and panics on errors as they do. In the JSONv2 mode it uses the
// encoding/json/v2 defaults plus the operation's ResponseOptions, and
// returns errors so that the field resolves to null with an error.
func writeJSONContext(ctx context.Context, w io.Writer, v any) error {
	m := GetJSONMode(ctx)
	if m == nil || m.Version != JSONv2 {
		if err := jsonv1.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
		return nil
	}

	var opts []json.Options
	if o := m.ResponseOptions; o != nil {
		opts = append(opts, o)
	}
	// Marshal to a buffer first so nothing is written when it fails.
	b, err := json.Marshal(v, opts...)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
