package exec

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

func render(m graphql.Marshaler) string {
	var buf bytes.Buffer
	m.MarshalGQL(&buf)
	return buf.String()
}

func responseCtx() context.Context {
	ctx := graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, nil)
	return graphql.WithFieldContext(ctx, &graphql.FieldContext{})
}

func TestMarshalNull(t *testing.T) {
	ctx := responseCtx()
	m := MarshalFuncPtr[*testEC](false, graphql.MarshalString)
	require.Equal(t, graphql.Null, m(ctx, nil, nil, nil))
	require.Empty(t, graphql.GetErrors(ctx))

	m = MarshalFuncPtr[*testEC](true, graphql.MarshalString)
	require.Equal(t, graphql.Null, m(ctx, nil, nil, nil))
	require.Len(t, graphql.GetErrors(ctx), 1)
	require.Equal(
		t,
		"the requested element is null which the schema does not allow",
		graphql.GetErrors(ctx)[0].Message,
	)

	s := "x"
	require.Equal(t, `"x"`, render(m(ctx, nil, nil, &s)))
}

func TestMarshalList(t *testing.T) {
	ctx := responseCtx()
	elem := MarshalFuncPtr[*testEC](true, graphql.MarshalString)
	s := "a"

	require.Equal(
		t,
		`["a"]`,
		render(MarshalList(elem, ListOptions{Leaf: true})(ctx, nil, nil, []*string{&s})),
	)
	require.Equal(
		t,
		graphql.Null,
		MarshalList(elem, ListOptions{Nullable: true})(ctx, nil, nil, nil),
	)
	require.Equal(t, `[]`, render(MarshalList(elem, ListOptions{})(ctx, nil, nil, nil)))
	// A null element of a list of non-null elements makes the list null.
	require.Equal(
		t,
		graphql.Null,
		MarshalList(
			elem,
			ListOptions{ElemNonNull: true, Leaf: true},
		)(
			ctx,
			nil,
			nil,
			[]*string{&s, nil},
		),
	)
	require.Equal(
		t,
		`["a",null]`,
		render(
			MarshalList(
				MarshalFuncPtr[*testEC](false, graphql.MarshalString),
				ListOptions{Leaf: true},
			)(
				ctx,
				nil,
				nil,
				[]*string{&s, nil},
			),
		),
	)
}

func TestUnmarshalCombinators(t *testing.T) {
	ctx := context.Background()

	list, err := UnmarshalList(true, unmarshalInt)(ctx, nil, 3)
	require.NoError(t, err)
	require.Equal(t, []int{3}, list)
	list, err = UnmarshalList(true, unmarshalInt)(ctx, nil, nil)
	require.NoError(t, err)
	require.Nil(t, list)
	_, err = UnmarshalList(false, unmarshalInt)(ctx, nil, []any{1, "two"})
	require.EqualError(t, err, "not an int")

	in := pointInput()
	p, err := UnmarshalInputPtr[*testEC, point](true, in)(ctx, nil, nil)
	require.NoError(t, err)
	require.Nil(t, p)
	p, err = UnmarshalInputPtr[*testEC, point](false, in)(ctx, nil, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, &point{X: 1, Y: 7}, p)

	sp, err := UnmarshalFuncPtr[*testEC](true, graphql.UnmarshalString)(ctx, nil, nil)
	require.NoError(t, err)
	require.Nil(t, sp)
	sp, err = UnmarshalFuncPtr[*testEC](false, graphql.UnmarshalString)(ctx, nil, "s")
	require.NoError(t, err)
	require.Equal(t, "s", *sp)
}
