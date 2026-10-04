package exec

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

func TestChain(t *testing.T) {
	var log []string
	tag := &DirectiveDef[*testEC]{
		Name: "tag",
		Args: []Arg[*testEC]{
			{Name: "name", Type: InOf(func(_ context.Context, _ *testEC, v any) (string, error) {
				if v == "bad" {
					return "", errors.New("bad name")
				}
				return fmt.Sprint(v), nil
			})},
		},
		Call: func(ctx context.Context, ec *testEC, obj any, next graphql.Resolver, args map[string]any) (any, error) {
			log = append(log, "before "+args["name"].(string))
			res, err := next(ctx)
			log = append(log, "after "+args["name"].(string))
			return fmt.Sprintf("%s(%v)", args["name"], res), err
		},
	}
	next := func(ctx context.Context) (any, error) { return "value", nil }

	res, err := Chain(
		nil,
		nil,
		Dirs(tag.With(map[string]any{"name": "inner"}), tag.With(map[string]any{"name": "outer"})),
		next,
	)(
		context.Background(),
	)
	require.NoError(t, err)
	require.Equal(t, "outer(inner(value))", res)
	require.Equal(t, []string{"before outer", "before inner", "after inner", "after outer"}, log)

	_, err = Chain(
		nil,
		nil,
		Dirs(tag.With(map[string]any{"name": "bad"})),
		next,
	)(
		context.Background(),
	)
	require.EqualError(t, err, "bad name")

	res, err = Chain[*testEC](nil, nil, nil, next)(context.Background())
	require.NoError(t, err)
	require.Equal(t, "value", res)
}
