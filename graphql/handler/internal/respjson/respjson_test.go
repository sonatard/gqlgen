package respjson

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

// fakeExecutor carries a JSON mode and response options, as
// *executor.Executor does.
type fakeExecutor struct {
	graphql.GraphExecutor
	version graphql.JSONVersion
	opts    json.Options
}

func (e fakeExecutor) JSONVersion() graphql.JSONVersion  { return e.version }
func (e fakeExecutor) ResponseJSONOptions() json.Options { return e.opts }

func TestMarshalFollowsJSONVersion(t *testing.T) {
	// HTML characters, a nil slice and two map keys show which package and
	// options wrote the response.
	resp := &graphql.Response{
		Data:       []byte(`{"a":"<b>"}`),
		Extensions: map[string]any{"z": []int(nil), "a": 1},
	}
	v1, err := jsonv1.Marshal(resp)
	require.NoError(t, err)
	v2, err := json.Marshal(resp, json.Deterministic(true))
	require.NoError(t, err)
	v2Escaped, err := json.Marshal(resp, json.Deterministic(true), jsontext.EscapeForHTML(true))
	require.NoError(t, err)

	require.Contains(t, string(v1), `"\u003cb\u003e"`)
	require.Contains(t, string(v2), `"<b>"`)
	require.Contains(t, string(v2Escaped), `"\u003cb\u003e"`)

	tests := []struct {
		name string
		exec graphql.GraphExecutor
		want []byte
	}{
		{name: "no executor", exec: nil, want: v1},
		{name: "executor without a mode", exec: struct{ graphql.GraphExecutor }{}, want: v1},
		{name: "v1", exec: fakeExecutor{version: graphql.JSONv1}, want: v1},
		{
			name: "v1 ignores response options",
			exec: fakeExecutor{version: graphql.JSONv1, opts: jsontext.EscapeForHTML(false)},
			want: v1,
		},
		{
			name: "v2",
			exec: fakeExecutor{version: graphql.JSONv2, opts: json.Deterministic(true)},
			want: v2,
		},
		{
			name: "v2 adds response options to its defaults",
			exec: fakeExecutor{
				version: graphql.JSONv2,
				opts:    json.JoinOptions(json.Deterministic(true), jsontext.EscapeForHTML(true)),
			},
			want: v2Escaped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.exec, resp)
			require.NoError(t, err)
			require.Equal(t, string(tt.want), string(got))
		})
	}

	t.Run("v2 without options", func(t *testing.T) {
		got, err := Marshal(fakeExecutor{version: graphql.JSONv2}, resp)
		require.NoError(t, err)
		require.Contains(t, string(got), `"<b>"`)
		require.Contains(t, string(got), `"z":[]`)
	})
}
