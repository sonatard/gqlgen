package execbehavior

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestComplexity(t *testing.T) {
	cfg := Config{Resolvers: &Stub{}}
	cfg.Complexity.Query.LimitedItems = func(childComplexity, count int) int {
		return count * childComplexity
	}
	var gotDefault *int
	cfg.Complexity.Query.ArgProbe = func(childComplexity int, strict *Strict, withDefault *int, plain *string) int {
		gotDefault = withDefault
		return 100
	}
	cfg.Complexity.ArgObject.Echo = func(childComplexity int, strict Strict) int {
		return 50
	}
	directiveCalls := 0
	cfg.Directives.ReturnValue = func(ctx context.Context, obj any, next graphql.Resolver, kind string) (any, error) {
		directiveCalls++
		return next(ctx)
	}
	var wrongTypeArgCalls int
	cfg.Complexity.Query.WrongTypeArg = func(childComplexity int, value *string) int {
		wrongTypeArgCalls++
		return 1000
	}
	// With a limit of 1 every operation fails, and the error says its complexity.
	srv := handler.New(NewExecutableSchema(cfg))
	srv.AddTransport(transport.POST{})
	srv.Use(extension.FixedComplexityLimit(1))
	complexity := func(t *testing.T, query string) string {
		t.Helper()
		return post(t, srv, query)
	}
	exceeds := func(n string) string {
		return `{"data":null,"errors":[{"message":"operation has complexity ` + n +
			`, which exceeds the limit of 1","extensions":{"code":"COMPLEXITY_LIMIT_EXCEEDED"}}]}`
	}

	t.Run("default complexity counts each field", func(t *testing.T) {
		require.JSONEq(t, exceeds("4"), complexity(t, `{ viewer { name query { ping } } }`))
	})

	t.Run("complexity function with an argument", func(t *testing.T) {
		require.JSONEq(t, exceeds("16"), complexity(t, `{ limitedItems(count: 8) { id slow } }`))
	})

	t.Run("complexity function receives argument defaults", func(t *testing.T) {
		require.JSONEq(t, exceeds("100"), complexity(t, `{ argProbe }`))
		require.Equal(t, 7, *gotDefault)
	})

	t.Run("nested complexity function", func(t *testing.T) {
		require.JSONEq(t, exceeds("52"), complexity(t, `{ argObject { ok echo(strict: "s") } }`))
	})

	t.Run("arguments that fail to unmarshal fall back to the default", func(t *testing.T) {
		require.JSONEq(t, exceeds("3"), complexity(t, `{ argObject { ok echo(strict: "bad") } }`))
	})

	t.Run("argument directives run when complexity parses arguments", func(t *testing.T) {
		directiveCalls, wrongTypeArgCalls = 0, 0
		require.JSONEq(t, exceeds("1000"), complexity(t, `{ wrongTypeArg(value: "v") }`))
		require.Equal(t, 1, directiveCalls)
		require.Equal(t, 1, wrongTypeArgCalls)
	})
}
