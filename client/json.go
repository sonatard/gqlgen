package client

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"

	"github.com/99designs/gqlgen/graphql"
)

// response is Response with the names servers write. encoding/json/v2
// matches names case-sensitively, so the untagged fields of Response would
// not match them.
type response struct {
	Data       any             `json:"data"`
	Errors     json.RawMessage `json:"errors"`
	Extensions map[string]any  `json:"extensions"`
}

// marshalJSON marshals v with the JSON package version selects.
func marshalJSON(version graphql.JSONVersion, v any) ([]byte, error) {
	if version == graphql.JSONv2 {
		return jsonv2.Marshal(v)
	}
	return json.Marshal(v)
}

// unmarshalJSON unmarshals data into v with the JSON package version selects.
func unmarshalJSON(version graphql.JSONVersion, data []byte, v any) error {
	if version == graphql.JSONv2 {
		return jsonv2.Unmarshal(data, v)
	}
	return json.Unmarshal(data, v)
}

// unmarshalResponse unmarshals a GraphQL response with the JSON package
// version selects.
func unmarshalResponse(version graphql.JSONVersion, data []byte) (*Response, error) {
	var r response
	if err := unmarshalJSON(version, data, &r); err != nil {
		return nil, err
	}
	resp := Response(r)
	return &resp, nil
}
