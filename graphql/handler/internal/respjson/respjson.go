// Package respjson encodes the JSON that the handler and its transports
// write to clients: GraphQL responses, error lists and connection payloads.
package respjson

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"

	"github.com/99designs/gqlgen/graphql"
)

// jsonVersioned and optionsCarrier are implemented by executors that carry
// the JSON mode and the response options, such as *executor.Executor.
type (
	jsonVersioned interface {
		JSONVersion() graphql.JSONVersion
	}
	optionsCarrier interface {
		ResponseJSONOptions() json.Options
	}
)

// Marshal returns the JSON encoding of v in exec's JSON mode: with
// encoding/json in the JSONv1 mode, and with encoding/json/v2's defaults plus
// any options set with SetResponseJSONOptions in the JSONv2 mode. A nil exec,
// or one that carries no mode, uses JSONv1.
func Marshal(exec graphql.GraphExecutor, v any) ([]byte, error) {
	if !isJSONv2(exec) {
		return jsonv1.Marshal(v)
	}
	if c, ok := exec.(optionsCarrier); ok {
		if opts := c.ResponseJSONOptions(); opts != nil {
			return json.Marshal(v, opts)
		}
	}
	return json.Marshal(v)
}

func isJSONv2(exec graphql.GraphExecutor) bool {
	v, ok := exec.(jsonVersioned)
	return ok && v.JSONVersion() == graphql.JSONv2
}
