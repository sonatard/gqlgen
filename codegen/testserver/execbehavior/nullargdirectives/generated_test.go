//go:generate go run ../../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go

package nullargdirectives

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestArgumentDirectivesWithNull(t *testing.T) {
	resolvers := &Stub{}
	resolvers.QueryResolver.Echo = func(ctx context.Context, arg *string) (*string, error) {
		return arg, nil
	}
	// received describes what the directive received from the argument.
	var received []string
	srv := handler.New(NewExecutableSchema(Config{
		Resolvers: resolvers,
		Directives: DirectiveRoot{
			Fill: func(ctx context.Context, obj any, next graphql.Resolver, value string) (any, error) {
				res, err := next(ctx)
				if err != nil {
					return nil, err
				}
				if s, ok := res.(*string); ok && s != nil {
					received = append(received, *s)
					return s, nil
				}
				received = append(received, fmt.Sprintf("%T(nil)", res))
				return &value, nil
			},
		},
	}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	for _, tc := range []struct{ query, want string }{
		{`{ echo(arg: "given") }`, "given"},
		{`{ echo }`, "filled"},
		{`{ echo(arg: null) }`, "filled"},
	} {
		var resp struct{ Echo string }
		require.NoError(t, c.Post(tc.query, &resp), tc.query)
		require.Equal(t, tc.want, resp.Echo, tc.query)
	}
	require.Equal(t, []string{"given", "*string(nil)", "*string(nil)"}, received)
}

func TestDirectiveArgumentOfInterfaceType(t *testing.T) {
	resolvers := &Stub{}
	resolvers.QueryResolver.Tagged = func(ctx context.Context) (*string, error) {
		s := "tagged"
		return &s, nil
	}
	resolvers.QueryResolver.TaggedWith = func(ctx context.Context) (*string, error) {
		s := "taggedWith"
		return &s, nil
	}
	// received holds, by field, what the directive received as its argument.
	received := map[string]string{}
	var mu sync.Mutex
	srv := handler.New(NewExecutableSchema(Config{
		Resolvers: resolvers,
		Directives: DirectiveRoot{
			Tag: func(ctx context.Context, obj any, next graphql.Resolver, tag Tag) (any, error) {
				got := "nil"
				if tag != nil {
					got = tag.Tag()
				}
				mu.Lock()
				received[graphql.GetFieldContext(ctx).Field.Name] = got
				mu.Unlock()
				return next(ctx)
			},
		},
	}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	var resp struct{ Tagged, TaggedWith string }
	require.NoError(t, c.Post(`{ tagged taggedWith }`, &resp))
	require.Equal(t, "tagged", resp.Tagged)
	require.Equal(t, "taggedWith", resp.TaggedWith)
	// The absent argument is nil, not a panic on a type assertion of nil.
	require.Equal(t, map[string]string{"tagged": "nil", "taggedWith": "given"}, received)
}
