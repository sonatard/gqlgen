package respjson

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/99designs/gqlgen/graphql"
)

// TestMarshalMatchesEncodingJSON pins Marshal to encoding/json.Marshal, which
// is what the handler and the transports wrote with before: for every value
// both either fail with the same message or produce the same bytes.
func TestMarshalMatchesEncodingJSON(t *testing.T) {
	hasNext := false
	values := []any{
		// Data is copied, but re-escaped and compacted the way encoding/json
		// does it.
		&graphql.Response{Data: []byte(
			"{\"a\":\"<b> & </b>\",\"s\":\"\u2028\u2029\",\"n\":1e400,\"f\":1.0}",
		)},
		&graphql.Response{Data: []byte("{\"a\":\"\xff\",\"a\":1}")},
		&graphql.Response{Data: []byte("{\"m\":{\"x\":1}\n,\"esc\":\"\\/\\u00e9\"}")},
		&graphql.Response{Data: []byte(`{"unterminated":`)},
		&graphql.Response{Data: []byte(`not json`)},
		&graphql.Response{},
		&graphql.Response{
			Errors: gqlerror.List{
				{
					Message:   "boom <x> & \u2028",
					Path:      ast.Path{ast.PathName("a"), ast.PathIndex(1)},
					Locations: []gqlerror.Location{{Line: 1, Column: 2}},
					Extensions: map[string]any{
						"code":  "INTERNAL",
						"after": time.Second,
						"raw":   []byte("hi"),
					},
				},
			},
			Data:    []byte(`null`),
			Label:   "deferred",
			Path:    ast.Path{ast.PathName("items"), ast.PathIndex(0)},
			HasNext: &hasNext,
			Extensions: map[string]any{
				"nilSlice": []int(nil),
				"nilMap":   map[string]int(nil),
				"number":   jsonv1.Number("1.50"),
			},
		},
		&graphql.Response{Extensions: map[string]any{}},
		&graphql.Response{Extensions: map[string]any{"bad": jsonv1.Number("007")}},
		&graphql.Response{Extensions: map[string]any{"bad": make(chan int)}},
		struct {
			Incremental []*graphql.Response `json:"incremental"`
			HasNext     bool                `json:"hasNext"`
		}{Incremental: []*graphql.Response{{Data: []byte(`{"a":1}`), Path: ast.Path{ast.PathName("x")}}}},
		[]error{&gqlerror.Error{Message: "a"}, &gqlerror.Error{Message: "b"}},
		&gqlerror.Error{Message: "connection <closed>"},
		map[string]any{"ack": true, "z": nil, "a": []any{1, "x"}},
	}

	for i, v := range values {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			requireMarshalsLikeEncodingJSON(t, v)
		})
	}
}

// FuzzMarshalMatchesEncodingJSON checks the same property for responses built
// from fuzzed data and error messages.
func FuzzMarshalMatchesEncodingJSON(f *testing.F) {
	f.Add([]byte(`{"a":"<b>"}`), "boom <x>")
	f.Add([]byte("{\"a\":\"\xff\"}"), "\xff")
	f.Add([]byte("{\"a\":1}\n"), "\u2028")
	f.Add([]byte(`{"a":`), "")
	f.Fuzz(func(t *testing.T, data []byte, message string) {
		requireMarshalsLikeEncodingJSON(t, &graphql.Response{
			Errors: gqlerror.List{{Message: message}},
			Data:   data,
		})
	})
}

func requireMarshalsLikeEncodingJSON(t *testing.T, v any) {
	t.Helper()

	want, wantErr := jsonv1.Marshal(v)
	got, gotErr := Marshal(nil, v)
	if wantErr != nil {
		require.EqualError(t, gotErr, wantErr.Error())
		require.Equal(t, fmt.Sprintf("%T", wantErr), fmt.Sprintf("%T", gotErr))
		return
	}
	require.NoError(t, gotErr)
	require.Equal(t, string(want), string(got))
}

type executorWithOptions struct {
	graphql.GraphExecutor
	opts json.Options
}

func (e executorWithOptions) ResponseJSONOptions() json.Options { return e.opts }

func TestMarshalUsesExecutorOptions(t *testing.T) {
	const escaped = `{"data":{"a":"\u003cb\u003e"}}`
	tests := []struct {
		name string
		exec graphql.GraphExecutor
		want string
	}{
		{name: "no executor", exec: nil, want: escaped},
		{name: "executor without options", exec: struct{ graphql.GraphExecutor }{}, want: escaped},
		{name: "executor with nil options", exec: executorWithOptions{}, want: escaped},
		{
			name: "executor with encoding/json/v2 defaults",
			exec: executorWithOptions{opts: json.DefaultOptionsV2()},
			want: `{"data":{"a":"<b>"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.exec, &graphql.Response{Data: []byte(`{"a":"<b>"}`)})
			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}
