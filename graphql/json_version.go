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

// GetJSONVersion returns the JSON version of the operation in ctx, or JSONv1
// if ctx carries no operation.
func GetJSONVersion(ctx context.Context) JSONVersion {
	if !HasOperationContext(ctx) {
		return JSONv1
	}
	return GetOperationContext(ctx).JSONVersion
}

// writeJSONContext writes v with the JSON package the operation in ctx
// selects. In the JSONv1 mode it writes what MarshalAny and MarshalMap write,
// and panics on errors as they do. In the JSONv2 mode it uses the
// encoding/json/v2 defaults plus the operation's ResponseJSONOptions, and
// returns errors so that the field resolves to null with an error.
func writeJSONContext(ctx context.Context, w io.Writer, v any) error {
	if GetJSONVersion(ctx) != JSONv2 {
		if err := jsonv1.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
		return nil
	}

	var opts []json.Options
	if o := GetOperationContext(ctx).ResponseJSONOptions; o != nil {
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
