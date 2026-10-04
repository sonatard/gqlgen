package transport

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

// jsonV2Exec is an executor in the JSONv2 mode. jsonDecode only asks it for
// the mode.
type jsonV2Exec struct{ graphql.GraphExecutor }

func (jsonV2Exec) JSONVersion() graphql.JSONVersion { return graphql.JSONv2 }

const decodeBody = `{"query":"q","variables":{"big":9007199254740993,"f":-1.5e3,"s":"x",` +
	`"t":true,"z":null,"l":[1,"a",[]],"o":{"k":2}}}`

func TestJSONDecodeV1(t *testing.T) {
	var params graphql.RawParams
	require.NoError(t, jsonDecode(nil, strings.NewReader(decodeBody), &params))
	assert.Equal(t, "q", params.Query)
	assert.Equal(t, map[string]any{
		"big": json.Number("9007199254740993"),
		"f":   json.Number("-1.5e3"),
		"s":   "x",
		"t":   true,
		"z":   nil,
		"l":   []any{json.Number("1"), "a", []any{}},
		"o":   map[string]any{"k": json.Number("2")},
	}, params.Variables)

	t.Run("encoding/json behavior", func(t *testing.T) {
		var params graphql.RawParams
		body := `{"Query":"a","query":"b"} trailing`
		require.NoError(t, jsonDecode(nil, strings.NewReader(body), &params))
		assert.Equal(t, "b", params.Query)
	})
}

func TestJSONDecodeV2(t *testing.T) {
	exec := jsonV2Exec{}

	var params graphql.RawParams
	require.NoError(t, jsonDecode(exec, strings.NewReader(decodeBody), &params))
	assert.Equal(t, "q", params.Query)
	assert.Equal(t, map[string]any{
		"big": jsontext.Value("9007199254740993"),
		"f":   jsontext.Value("-1.5e3"),
		"s":   "x",
		"t":   true,
		"z":   nil,
		"l":   []any{jsontext.Value("1"), "a", []any{}},
		"o":   map[string]any{"k": jsontext.Value("2")},
	}, params.Variables)

	t.Run("names match case-sensitively", func(t *testing.T) {
		var params graphql.RawParams
		require.NoError(t, jsonDecode(exec, strings.NewReader(`{"Query":"a"}`), &params))
		assert.Empty(t, params.Query)
	})

	for name, body := range map[string]string{
		"duplicate names":           `{"query":"a","query":"b"}`,
		"duplicate variable names":  `{"query":"a","variables":{"v":1,"v":2}}`,
		"trailing data":             `{"query":"a"} {"query":"b"}`,
		"invalid UTF-8":             "{\"query\":\"\xff\"}",
		"truncated":                 `{"query":"a"`,
		"invalid number":            `{"query":"a","variables":{"v":01}}`,
		"invalid literal in a list": `{"query":"a","variables":{"v":[tru]}}`,
	} {
		t.Run(name+" are errors", func(t *testing.T) {
			var params graphql.RawParams
			assert.Error(t, jsonDecode(exec, strings.NewReader(body), &params))
		})
	}

	t.Run("numbers keep their text past the read buffer", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(`{"query":"q","variables":{"l":[`)
		for i := range 5000 {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", 1_000_000_000+i)
		}
		b.WriteString(`]}}`)

		var params graphql.RawParams
		require.NoError(t, jsonDecode(exec, strings.NewReader(b.String()), &params))
		l := params.Variables["l"].([]any)
		require.Len(t, l, 5000)
		for i, v := range l {
			require.Equal(t, jsontext.Value(strconv.Itoa(1_000_000_000+i)), v)
		}
	})

	t.Run("typed destinations", func(t *testing.T) {
		uploads := map[string][]string{}
		body := `{"0":["variables.file"]}`
		require.NoError(t, jsonDecode(exec, strings.NewReader(body), &uploads))
		assert.Equal(t, map[string][]string{"0": {"variables.file"}}, uploads)
	})
}

func TestJSONUnmarshal(t *testing.T) {
	for _, exec := range []graphql.GraphExecutor{nil, jsonV2Exec{}} {
		payload := InitPayload{}
		require.NoError(t, jsonUnmarshal(exec, []byte(`{"n":1,"s":"x"}`), &payload))
		assert.Equal(t, InitPayload{"n": 1.0, "s": "x"}, payload)
	}

	payload := InitPayload{}
	require.NoError(t, jsonUnmarshal(nil, []byte(`{"n":1,"n":2}`), &payload))
	assert.Error(t, jsonUnmarshal(jsonV2Exec{}, []byte(`{"n":1,"n":2}`), &payload))
}

func BenchmarkJSONDecode(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"query":"query($in: [Input!]!) { f(in: $in) }","variables":{"in":[`)
	for i := range 500 {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(
			&sb,
			`{"id":%d,"name":"item %d","price":%d.5,"tags":["a","b"],"ok":true}`,
			i,
			i,
			i,
		)
	}
	sb.WriteString(`]}}`)
	body := sb.String()

	for _, bc := range []struct {
		name string
		exec graphql.GraphExecutor
	}{{"v1", nil}, {"v2", jsonV2Exec{}}} {
		b.Run(bc.name, func(b *testing.B) {
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			for b.Loop() {
				var params graphql.RawParams
				if err := jsonDecode(bc.exec, strings.NewReader(body), &params); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
