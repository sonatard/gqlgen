package graphql

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"io"
)

func MarshalAny(v any) Marshaler {
	return WriterFunc(func(w io.Writer) {
		b, err := json.Marshal(v, jsonv1.DefaultOptionsV1())
		if err != nil {
			panic(err)
		}
		// encoding/json's Encoder ended each value with a newline.
		w.Write(b)
		io.WriteString(w, "\n")
	})
}

func UnmarshalAny(v any) (any, error) {
	return v, nil
}
