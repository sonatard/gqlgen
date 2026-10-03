//go:generate go run ../../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go

package nullargdirectives

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/codegen/testserver/execbehavior/internal/exectest"
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

	for _, tc := range []struct{ query, want string }{
		{`{ echo(arg: "given") }`, `{"data":{"echo":"given"}}`},
		{`{ echo }`, `{"data":{"echo":"filled"}}`},
		{`{ echo(arg: null) }`, `{"data":{"echo":"filled"}}`},
	} {
		require.JSONEq(t, tc.want, exectest.Post(t, srv, tc.query, nil), tc.query)
	}
	require.Equal(t, []string{"given", "*string(nil)", "*string(nil)"}, received)
}
