package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

func MarshalMap(val map[string]any) Marshaler {
	return WriterFunc(func(w io.Writer) {
		err := json.NewEncoder(w).Encode(val)
		if err != nil {
			panic(err)
		}
	})
}

func UnmarshalMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}

	return nil, fmt.Errorf("%T is not a map", v)
}

// MarshalMapContext writes val with the JSON package the operation's JSON mode
// selects. In the JSONv1 mode it writes what MarshalMap writes. It is the
// marshaler gqlgen binds the built-in Map scalar to.
func MarshalMapContext(val map[string]any) ContextMarshaler {
	return ContextWriterFunc(func(ctx context.Context, w io.Writer) error {
		return writeJSONContext(ctx, w, val)
	})
}

func UnmarshalMapContext(_ context.Context, v any) (map[string]any, error) {
	return UnmarshalMap(v)
}
