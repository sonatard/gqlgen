// Package respjson encodes the JSON that the handler and its transports
// write to clients: GraphQL responses, error lists and connection payloads.
package respjson

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"

	"github.com/99designs/gqlgen/graphql"
)

// defaultOptions are the encoding/json/v2 options responses are written with
// unless the executor carries others. jsonv1.DefaultOptionsV1 is what
// encoding/json.Marshal itself runs with, so the output is byte for byte what
// encoding/json.Marshal produces.
var defaultOptions = jsonv1.DefaultOptionsV1()

// optionsCarrier is implemented by executors that let users choose the
// options, such as *executor.Executor.
type optionsCarrier interface {
	ResponseJSONOptions() json.Options
}

// Marshal returns the JSON encoding of v with the options exec carries, or
// with the default options if exec is nil, carries none or carries nil.
func Marshal(exec graphql.GraphExecutor, v any) ([]byte, error) {
	return json.Marshal(v, options(exec))
}

func options(exec graphql.GraphExecutor) json.Options {
	if c, ok := exec.(optionsCarrier); ok {
		if opts := c.ResponseJSONOptions(); opts != nil {
			return opts
		}
	}
	return defaultOptions
}
