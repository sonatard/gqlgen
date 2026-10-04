package graphql

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encoding/json writes a nil slice as null and matches names case-insensitively;
// encoding/json/v2 writes [] and matches names exactly. The tests below use
// these differences to tell which package and options Omittable follows.

type omittableInner struct{ A int }

func TestOmittableMarshalJSONToFollowsCaller(t *testing.T) {
	type S struct {
		V Omittable[[]string] `json:",omitzero"`
	}
	set := S{V: OmittableOf[[]string](nil)}

	b, err := json.Marshal(set)
	require.NoError(t, err)
	assert.JSONEq(t, `{"V":[]}`, string(b))

	b, err = json.Marshal(set, jsonv1.DefaultOptionsV1())
	require.NoError(t, err)
	assert.JSONEq(t, `{"V":null}`, string(b))

	b, err = jsonv1.Marshal(set)
	require.NoError(t, err)
	assert.JSONEq(t, `{"V":null}`, string(b))

	b, err = json.Marshal(S{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(b))
}

func TestOmittableUnmarshalJSONFromFollowsCaller(t *testing.T) {
	type S struct {
		V Omittable[omittableInner]
	}

	for _, tc := range []struct {
		name      string
		unmarshal func([]byte, any) error
		in        string
		wantSet   bool
		want      omittableInner
	}{
		{"v2", func(b []byte, v any) error { return json.Unmarshal(b, v) }, `{"V":{"a":1}}`, true, omittableInner{}},
		{"v2 exact name", func(b []byte, v any) error { return json.Unmarshal(b, v) }, `{"V":{"A":1}}`, true, omittableInner{A: 1}},
		{"v2 null", func(b []byte, v any) error { return json.Unmarshal(b, v) }, `{"V":null}`, true, omittableInner{}},
		{"v2 absent", func(b []byte, v any) error { return json.Unmarshal(b, v) }, `{}`, false, omittableInner{}},
		{"v1", jsonv1.Unmarshal, `{"V":{"a":1}}`, true, omittableInner{A: 1}},
		{"v1 null", jsonv1.Unmarshal, `{"V":null}`, true, omittableInner{}},
		{"v1 absent", jsonv1.Unmarshal, `{}`, false, omittableInner{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s S
			require.NoError(t, tc.unmarshal([]byte(tc.in), &s))
			got, ok := s.V.ValueOK()
			assert.Equal(t, tc.wantSet, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOmittableContextMethodsFollowJSONVersion(t *testing.T) {
	withMode := func(m *JSONMode) context.Context {
		return WithOperationContext(context.Background(), &OperationContext{JSONMode: m})
	}

	for _, tc := range []struct {
		name          string
		ctx           context.Context
		wantMarshal   string
		wantUnmarshal omittableInner
	}{
		{"no operation", context.Background(), `null`, omittableInner{A: 1}},
		{"v1", withMode(&JSONMode{Version: JSONv1}), `null`, omittableInner{A: 1}},
		{"v2", withMode(&JSONMode{Version: JSONv2}), `[]`, omittableInner{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			OmittableOf[[]string](nil).MarshalGQLContext(tc.ctx, &b)
			assert.Equal(t, tc.wantMarshal, b.String())

			var o Omittable[omittableInner]
			require.NoError(t, o.UnmarshalGQLContext(tc.ctx, []byte(`{"a":1}`)))
			got, ok := o.ValueOK()
			assert.True(t, ok)
			assert.Equal(t, tc.wantUnmarshal, got)
		})
	}

	t.Run("v2 applies the response options", func(t *testing.T) {
		ctx := withMode(&JSONMode{Version: JSONv2, ResponseOptions: json.Deterministic(true)})
		var b bytes.Buffer
		OmittableOf(map[string]int{"c": 3, "a": 1, "b": 2}).MarshalGQLContext(ctx, &b)
		assert.Equal(t, `{"a":1,"b":2,"c":3}`, b.String())
	})

	t.Run("MarshalGQL and UnmarshalGQL stay on encoding/json", func(t *testing.T) {
		var b bytes.Buffer
		OmittableOf[[]string](nil).MarshalGQL(&b)
		assert.Equal(t, `null`, b.String())

		var o Omittable[omittableInner]
		require.NoError(t, o.UnmarshalGQL([]byte(`{"a":1}`)))
		assert.Equal(t, omittableInner{A: 1}, o.Value())
	})
}
