//go:generate rm -f resolver.go
//go:generate go run ../../../testdata/gqlgen.go -config gqlgen.yml -stub stub.go

package directiveargnames

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

// The generated code declares the arguments of a directive as variables beside the object
// it passes to the directive and the values it writes for the other arguments. Arguments
// with the names of those must not hide them.
func TestDirectiveArgumentNames(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []string
	)
	resolvers := &Stub{}
	resolvers.QueryResolver.Root = func(ctx context.Context) (*string, error) {
		return nil, nil
	}
	resolvers.QueryResolver.Probed = func(
		ctx context.Context, x *string, input *ProbeInput,
	) (*string, error) {
		return x, nil
	}
	srv := handler.New(NewExecutableSchema(Config{
		Resolvers: resolvers,
		Directives: DirectiveRoot{
			Probe: func(
				ctx context.Context, obj any, next graphql.Resolver,
				rawArgs, asMap, it, rawArgsArg, nilArg *string, trueArg, on *bool,
			) (any, error) {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, fmt.Sprintf(
					"obj=%v rawArgs=%s asMap=%s it=%s rawArgsArg=%s nil=%s true=%s on=%s",
					obj, deref(rawArgs), deref(asMap), deref(it), deref(rawArgsArg),
					deref(nilArg), deref(trueArg), deref(on),
				))
				return next(ctx)
			},
		},
	}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	t.Run("root field", func(t *testing.T) {
		calls = nil
		var resp struct{ Root *string }
		require.NoError(t, c.Post(`{ root }`, &resp))
		require.Equal(t, []string{
			"obj=<nil> rawArgs=nil asMap=nil it=nil rawArgsArg=nil nil=n true=nil on=nil",
		}, calls)
	})

	t.Run("argument and input object", func(t *testing.T) {
		calls = nil
		var resp struct{ Probed *string }
		require.NoError(t, c.Post(`{ probed(x: "a", input: {v: "b"}) }`, &resp))
		require.Equal(t, []string{
			"obj=map[input:map[v:b] x:a] rawArgs=r asMap=nil it=nil rawArgsArg=ra nil=n true=false on=true",
			"obj=map[v:b] rawArgs=nil asMap=m it=i rawArgsArg=nil nil=nil true=nil on=nil",
		}, calls)
	})
}

func deref[T any](p *T) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}
