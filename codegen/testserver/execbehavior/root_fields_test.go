package execbehavior

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

func TestRootFieldMiddleware(t *testing.T) {
	var (
		mu  sync.Mutex
		log []string
	)
	record := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		log = append(log, s)
	}
	resolvers := &Stub{}
	resolvers.QueryResolver.Ping = func(ctx context.Context) (string, error) {
		fc := graphql.GetFieldContext(ctx)
		record("resolve " + fc.Field.Name + " under " + fc.Parent.Object)
		return "pong", nil
	}
	var entries []string
	resolvers.MutationResolver.AppendLog = func(ctx context.Context, entry string) ([]string, error) {
		record("resolve appendLog " + entry)
		entries = append(entries, entry)
		return entries, nil
	}
	srv := newServer(resolvers, DirectiveRoot{})
	srv.AroundRootFields(func(ctx context.Context, next graphql.RootResolver) graphql.Marshaler {
		rc := graphql.GetRootFieldContext(ctx)
		record("root " + rc.Object + "=" + rc.Field.Alias)
		if rc.Field.Alias == "replaced" {
			return graphql.MarshalString("replaced by the root middleware")
		}
		return next(ctx)
	})
	srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (any, error) {
		fc := graphql.GetFieldContext(ctx)
		if fc.Parent.Object == "Query" || fc.Parent.Object == "Mutation" {
			record("field " + fc.Field.Alias)
		}
		return next(ctx)
	})

	t.Run("query", func(t *testing.T) {
		log = nil
		got := post(t, srv, `{ __typename ping replaced: ping }`)
		require.JSONEq(
			t,
			`{"data":{"__typename":"Query","ping":"pong","replaced":"replaced by the root middleware"}}`,
			got,
		)
		// Query fields resolve concurrently, so only the order within a field is fixed:
		// the root middleware wraps the field middleware, and replacing the result skips
		// the field. __typename does not run the middleware.
		require.ElementsMatch(t, []string{
			"root ping=ping", "field ping", "resolve ping under Query",
			"root ping=replaced",
		}, log)
		require.Less(t, slices.Index(log, "root ping=ping"), slices.Index(log, "field ping"))
		require.Less(
			t,
			slices.Index(log, "field ping"),
			slices.Index(log, "resolve ping under Query"),
		)
	})

	t.Run("mutation", func(t *testing.T) {
		// Mutation fields resolve one after the other, in the order of the query.
		log = nil
		got := post(
			t,
			srv,
			`mutation { first: appendLog(entry: "a") second: appendLog(entry: "b") }`,
		)
		require.JSONEq(t, `{"data":{"first":["a"],"second":["a","b"]}}`, got)
		require.Equal(t, []string{
			"root appendLog=first", "field first", "resolve appendLog a",
			"root appendLog=second", "field second", "resolve appendLog b",
		}, log)
	})
}
