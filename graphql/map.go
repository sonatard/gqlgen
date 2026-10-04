package graphql

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"io"
)

func MarshalMap(val map[string]any) Marshaler {
	return WriterFunc(func(w io.Writer) {
		b, err := json.Marshal(val, jsonv1.DefaultOptionsV1())
		if err != nil {
			panic(err)
		}
		// encoding/json's Encoder ended each value with a newline.
		w.Write(b)
		io.WriteString(w, "\n")
	})
}

func UnmarshalMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}

	return nil, fmt.Errorf("%T is not a map", v)
}
