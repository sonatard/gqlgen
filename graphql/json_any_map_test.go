package graphql

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnyAndMapFollowJSONVersion(t *testing.T) {
	val := map[string]any{"html": "<b>"}
	withOp := func(oc *OperationContext) context.Context {
		return WithOperationContext(context.Background(), oc)
	}

	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"no operation", context.Background(), `{"html":"\u003cb\u003e"}` + "\n"},
		{"v1", withOp(&OperationContext{JSONVersion: JSONv1}), `{"html":"\u003cb\u003e"}` + "\n"},
		{
			"v1 ignores the response options",
			withOp(&OperationContext{
				JSONVersion:         JSONv1,
				ResponseJSONOptions: jsontext.EscapeForHTML(false),
			}),
			`{"html":"\u003cb\u003e"}` + "\n",
		},
		{"v2", withOp(&OperationContext{JSONVersion: JSONv2}), `{"html":"<b>"}`},
		{
			"v2 with the response options",
			withOp(&OperationContext{
				JSONVersion:         JSONv2,
				ResponseJSONOptions: jsontext.EscapeForHTML(true),
			}),
			`{"html":"\u003cb\u003e"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cm2s(tc.ctx, t, MarshalAnyContext(val)))
			assert.Equal(t, tc.want, cm2s(tc.ctx, t, MarshalMapContext(val)))
		})
	}

	t.Run("v1 writes what MarshalAny and MarshalMap write", func(t *testing.T) {
		ctx := withOp(&OperationContext{JSONVersion: JSONv1})
		assert.Equal(t, m2s(MarshalAny(val)), cm2s(ctx, t, MarshalAnyContext(val)))
		assert.Equal(t, m2s(MarshalMap(val)), cm2s(ctx, t, MarshalMapContext(val)))
	})

	t.Run("v2 applies the response options", func(t *testing.T) {
		ctx := withOp(&OperationContext{
			JSONVersion:         JSONv2,
			ResponseJSONOptions: json.Deterministic(true),
		})
		m := map[string]any{"c": 3, "a": 1, "b": 2}
		assert.Equal(t, `{"a":1,"b":2,"c":3}`, cm2s(ctx, t, MarshalMapContext(m)))
	})

	t.Run("v2 returns errors and writes nothing", func(t *testing.T) {
		ctx := withOp(&OperationContext{JSONVersion: JSONv2})
		var b bytes.Buffer
		err := MarshalAnyContext("\xff").MarshalGQLContext(ctx, &b)
		require.Error(t, err)
		assert.Empty(t, b.String())
	})

	t.Run("v1 panics on errors as MarshalAny does", func(t *testing.T) {
		ctx := withOp(&OperationContext{JSONVersion: JSONv1})
		assert.Panics(t, func() { cm2s(ctx, t, MarshalAnyContext(func() {})) })
	})
}

func TestUnmarshalAnyAndMapContext(t *testing.T) {
	ctx := context.Background()

	v, err := UnmarshalAnyContext(ctx, jsontext.Value("1"))
	require.NoError(t, err)
	assert.Equal(t, jsontext.Value("1"), v)

	m, err := UnmarshalMapContext(ctx, map[string]any{"a": 1})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": 1}, m)

	_, err = UnmarshalMapContext(ctx, "a")
	require.EqualError(t, err, "string is not a map")
}

func cm2s(ctx context.Context, t *testing.T, m ContextMarshaler) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, m.MarshalGQLContext(ctx, &b))
	return b.String()
}
