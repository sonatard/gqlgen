// Package respjson encodes the JSON that the handler and its transports
// write to clients: GraphQL responses, error lists and connection payloads.
package respjson

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
)

// options are the encoding/json/v2 options responses are written with.
// jsonv1.DefaultOptionsV1 is what encoding/json.Marshal itself runs with, so
// the output is byte for byte what encoding/json.Marshal produces.
var options = jsonv1.DefaultOptionsV1()

// Marshal returns the JSON encoding of v.
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v, options)
}
