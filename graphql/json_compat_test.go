package graphql

import (
	"bytes"
	jsonv1 "encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The field marshalers and Omittable used to call encoding/json directly.
// These tests pin them to what encoding/json produces, byte for byte, now
// that they go through encoding/json/v2.

type compatStruct struct {
	A int    `json:"a,omitempty"`
	B string `json:"b"`
	C []int  `json:"c"`
	D *bool  `json:"d,omitempty"`
}

func compatValues() []any {
	return []any{
		nil,
		true,
		1.5,
		"<b> & </b>",
		string(rune(0x2028)) + string(rune(0x2029)),
		"invalid \xff utf-8",
		jsonv1.Number("1e400"),
		jsonv1.Number("007"),
		[]any{1, "x", nil, []int(nil)},
		map[string]any{"z": 1, "a": []int(nil), "m": map[string]int(nil), "<": ">"},
		compatStruct{},
		&compatStruct{A: 1, C: []int{}},
		time.Second,
		[]byte("hi"),
		[2]byte{1, 2},
		make(chan int),
		func() {},
	}
}

func TestMarshalAnyMatchesEncodingJSON(t *testing.T) {
	for i, v := range compatValues() {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			requireWritesLikeEncoder(t, v, MarshalAny(v))
		})
	}
}

func TestMarshalMapMatchesEncodingJSON(t *testing.T) {
	for i, v := range compatValues() {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			m := map[string]any{"v": v, "<k>": v}
			requireWritesLikeEncoder(t, m, MarshalMap(m))
		})
	}
	t.Run("nil map", func(t *testing.T) {
		requireWritesLikeEncoder(t, map[string]any(nil), MarshalMap(nil))
	})
}

// requireWritesLikeEncoder requires m to write what json.NewEncoder(w).Encode(v)
// writes, or to panic with the error Encode returns.
func requireWritesLikeEncoder(t *testing.T, v any, m Marshaler) {
	t.Helper()

	var want bytes.Buffer
	wantErr := jsonv1.NewEncoder(&want).Encode(v)

	var got bytes.Buffer
	var panicked any
	func() {
		defer func() { panicked = recover() }()
		m.MarshalGQL(&got)
	}()

	if wantErr != nil {
		require.NotNil(t, panicked, "encoding/json failed with %v", wantErr)
		gotErr, ok := panicked.(error)
		require.True(t, ok, "panicked with %T, not an error", panicked)
		require.EqualError(t, gotErr, wantErr.Error())
		require.Equal(t, fmt.Sprintf("%T", wantErr), fmt.Sprintf("%T", gotErr))
		require.Empty(t, got.String())
		return
	}
	require.Nil(t, panicked)
	require.Equal(t, want.String(), got.String())
}

func TestOmittableMatchesEncodingJSON(t *testing.T) {
	inputs := []string{
		`null`, `{"a":1,"A":2,"c":[1]}`, `{"b":"x","b":"y"}`, `[1,2]`, `"<b>"`, `1.5`,
		`{"a":`, `{"a":"not a number"}`, "\"\xff\"",
	}
	t.Run("map", func(t *testing.T) {
		requireOmittableLikeEncodingJSON(
			t,
			map[string]any{"<": nil, "n": jsonv1.Number("1")},
			inputs,
		)
	})
	t.Run("struct", func(t *testing.T) {
		requireOmittableLikeEncodingJSON(t, compatStruct{B: "<b>"}, inputs)
	})
	t.Run("pointer", func(t *testing.T) {
		s := "<b>"
		requireOmittableLikeEncodingJSON(t, &s, inputs)
	})
	t.Run("slice", func(t *testing.T) {
		requireOmittableLikeEncodingJSON(t, []int(nil), inputs)
	})
	t.Run("any", func(t *testing.T) {
		requireOmittableLikeEncodingJSON[any](t, time.Second, inputs)
	})
}

func requireOmittableLikeEncodingJSON[T any](t *testing.T, value T, inputs []string) {
	t.Helper()

	for _, o := range []Omittable[T]{OmittableOf(value), {}} {
		want, wantErr := jsonv1.Marshal(o.Value())

		got, gotErr := o.MarshalJSON()
		requireSameResult(t, want, wantErr, got, gotErr)

		var gql bytes.Buffer
		o.MarshalGQL(&gql)
		if wantErr == nil {
			require.Equal(t, string(want), gql.String())
		}
	}

	for _, input := range inputs {
		var want T
		wantErr := jsonv1.Unmarshal([]byte(input), &want)

		var got Omittable[T]
		gotErr := got.UnmarshalJSON([]byte(input))

		if wantErr != nil {
			require.EqualError(t, gotErr, wantErr.Error(), "input %q", input)
			continue
		}
		require.NoError(t, gotErr, "input %q", input)
		require.Equal(t, want, got.Value(), "input %q", input)
		require.True(t, got.IsSet())
	}
}

func requireSameResult(t *testing.T, want []byte, wantErr error, got []byte, gotErr error) {
	t.Helper()
	if wantErr != nil {
		require.EqualError(t, gotErr, wantErr.Error())
		return
	}
	require.NoError(t, gotErr)
	require.Equal(t, string(want), string(got))
}
