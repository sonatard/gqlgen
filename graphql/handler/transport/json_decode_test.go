package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

// These tests pin the contract every transport gets from jsonDecode.
func TestJSONDecode(t *testing.T) {
	t.Run("numbers stay exact at every depth", func(t *testing.T) {
		params := decodeParams(t, `{
			"query":"{ a }",
			"variables":{"n":9007199254740993,"f":1.5,"big":1e400,"list":[1,{"m":-0}]},
			"extensions":{"v":1}
		}`)
		assert.Equal(t, "{ a }", params.Query)
		assert.Equal(t, map[string]any{
			"n":    json.Number("9007199254740993"),
			"f":    json.Number("1.5"),
			"big":  json.Number("1e400"),
			"list": []any{json.Number("1"), map[string]any{"m": json.Number("-0")}},
		}, params.Variables)
		assert.Equal(t, map[string]any{"v": json.Number("1")}, params.Extensions)
	})

	t.Run("other scalars keep their Go types", func(t *testing.T) {
		params := decodeParams(t,
			`{"variables":{"s":"x","t":true,"f":false,"z":null,"o":{},"l":[]}}`)
		assert.Equal(t, map[string]any{
			"s": "x",
			"t": true,
			"f": false,
			"z": nil,
			"o": map[string]any{},
			"l": []any{},
		}, params.Variables)
	})

	t.Run("containers nest and strings unescape", func(t *testing.T) {
		params := decodeParams(t,
			`{"variables":{"a":[{},[],{"b":[1,"x",null,true,{"c":{}}]}],"s":"\u00e9\n\"q\""}}`)
		assert.Equal(t, map[string]any{
			"a": []any{
				map[string]any{},
				[]any{},
				map[string]any{"b": []any{
					json.Number("1"), "x", nil, true, map[string]any{"c": map[string]any{}},
				}},
			},
			"s": "\u00e9\n\"q\"",
		}, params.Variables)
	})

	t.Run("last duplicate member wins", func(t *testing.T) {
		params := decodeParams(t, `{"query":"{ a }","query":"{ b }","variables":{"x":1,"x":2}}`)
		assert.Equal(t, "{ b }", params.Query)
		assert.Equal(t, map[string]any{"x": json.Number("2")}, params.Variables)
	})

	t.Run("member names match fields case-insensitively", func(t *testing.T) {
		params := decodeParams(t, `{"Query":"{ a }","OPERATIONNAME":"Q"}`)
		assert.Equal(t, "{ a }", params.Query)
		assert.Equal(t, "Q", params.OperationName)
	})

	t.Run("underscores and dashes are not ignored when matching names", func(t *testing.T) {
		params := decodeParams(t, `{"operation_name":"Q","operation-name":"R"}`)
		assert.Empty(t, params.OperationName)
	})

	t.Run("invalid UTF-8 is replaced rather than rejected", func(t *testing.T) {
		params := decodeParams(t, "{\"query\":\"\xff\"}")
		assert.Equal(t, "\uFFFD", params.Query)
	})

	t.Run("only the first value is read", func(t *testing.T) {
		params := decodeParams(t, `{"query":"{ a }"} {"query":"{ b }"}`)
		assert.Equal(t, "{ a }", params.Query)

		params = decodeParams(t, `{"query":"{ a }"}trailing garbage`)
		assert.Equal(t, "{ a }", params.Query)
	})

	t.Run("null leaves the parameters empty", func(t *testing.T) {
		params := decodeParams(t, `null`)
		assert.Equal(t, graphql.RawParams{}, params)
	})

	t.Run("malformed input is an error", func(t *testing.T) {
		for _, body := range []string{
			``,
			`{`,
			`{"query":`,
			`notjson`,
			`{"query":"{ a }",}`,
			`[]`,
			`"{ a }"`,
			`{"query":123}`,
			`{"variables":"not an object"}`,
			`{"variables":{"x":1,}}`,
			`{"variables":{"x":}}`,
			`{"variables":{"x":1 "y":2}}`,
			`{"variables":{"x":[1,]}}`,
			`{"variables":{"x":[1}}`,
			`{"variables":{"x":tru}}`,
			`{"extensions":{"x":1}`,
		} {
			var params graphql.RawParams
			assert.Errorf(t, jsonDecode(strings.NewReader(body), &params), "body %q", body)
		}
	})
}

func decodeParams(t *testing.T, body string) graphql.RawParams {
	t.Helper()
	var params graphql.RawParams
	require.NoError(t, jsonDecode(strings.NewReader(body), &params))
	return params
}

func BenchmarkJSONDecode(b *testing.B) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"small", benchmarkRequestBody(3, 1)},
		{"large", benchmarkRequestBody(2000, 500)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.body)))
			b.ReportAllocs()
			for b.Loop() {
				var params graphql.RawParams
				if err := jsonDecode(bytes.NewReader(tc.body), &params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchmarkRequestBody builds a POST body with a list of numeric IDs and a
// list of input objects in its variables, the shape where decoding cost is
// dominated by the Variables map rather than the query string.
func benchmarkRequestBody(ids, inputs int) []byte {
	const query = `query Q($ids: [ID!]!, $inputs: [Input!]!, $first: Int) ` +
		`{ items(ids: $ids, inputs: $inputs, first: $first) { id name } }`

	var b strings.Builder
	fmt.Fprintf(&b, `{"query":%q,"operationName":"Q",`, query)
	b.WriteString(`"variables":{"first":10,"ids":[`)
	for i := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d", 9007199254740000+i)
	}
	b.WriteString(`],"inputs":[`)
	for i := range inputs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(
			&b,
			`{"name":"item %d","count":%d,"ratio":%g,"enabled":%t,"nested":{"a":1,"b":"x","c":null,"tags":["t1","t2"]}}`,
			i,
			i,
			float64(i)/3,
			i%2 == 0,
		)
	}
	b.WriteString(
		`]},"extensions":{"persistedQuery":{"version":1,"sha256Hash":"ecf4edb46db40b5132295c0291d62fb65d6759a9eedfa4d5d612dd5ec54a6b38"}}}`,
	)
	return []byte(b.String())
}
