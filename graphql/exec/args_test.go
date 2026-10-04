package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArgs(t *testing.T) {
	ctx := context.Background()
	args := []Arg[*testEC]{
		{Name: "x", Type: inInt},
		{Name: "note", Type: inString},
	}

	got, err := ParseArgs(ctx, nil, args, map[string]any{"x": 3})
	require.NoError(t, err)
	// Absent arguments hold the zero value of their Go type.
	require.Equal(t, map[string]any{"x": 3, "note": (*string)(nil)}, got)

	_, err = ParseArgs(ctx, nil, args, map[string]any{"x": "three"})
	require.EqualError(t, err, "not an int")
}

func TestParseArgsDirectives(t *testing.T) {
	ctx := context.Background()
	var calls int
	count := func(obj any) {
		calls++
		require.IsType(t, map[string]any{}, obj)
	}
	args := []Arg[*testEC]{
		{Name: "note", NilOK: true, Directives: returning(nil, count), Type: inString},
	}
	got, err := ParseArgs(ctx, nil, args, map[string]any{"note": "x"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"note": (*string)(nil)}, got)
	require.Equal(t, 1, calls)

	// Directives are skipped for absent arguments unless WithNull is set.
	_, err = ParseArgs(ctx, nil, args, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	args[0].WithNull = true
	_, err = ParseArgs(ctx, nil, args, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 2, calls)

	args = []Arg[*testEC]{{Name: "x", Directives: returning(nil, count), Type: inInt}}
	_, err = ParseArgs(ctx, nil, args, map[string]any{"x": 1})
	require.EqualError(t, err, "input: x unexpected type <nil> from directive, should be int")

	args[0].Directives = returning(8, count)
	got, err = ParseArgs(ctx, nil, args, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"x": 8}, got)
}
