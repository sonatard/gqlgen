package graphql

import (
	"context"
	"encoding/json"
	"io"
)

func MarshalAny(v any) Marshaler {
	return WriterFunc(func(w io.Writer) {
		err := json.NewEncoder(w).Encode(v)
		if err != nil {
			panic(err)
		}
	})
}

func UnmarshalAny(v any) (any, error) {
	return v, nil
}

// MarshalAnyContext writes v with the JSON package the operation's JSON mode
// selects. In the JSONv1 mode it writes what MarshalAny writes. It is the
// marshaler gqlgen binds the built-in Any scalar to.
func MarshalAnyContext(v any) ContextMarshaler {
	return ContextWriterFunc(func(ctx context.Context, w io.Writer) error {
		return writeJSONContext(ctx, w, v)
	})
}

func UnmarshalAnyContext(_ context.Context, v any) (any, error) {
	return UnmarshalAny(v)
}
