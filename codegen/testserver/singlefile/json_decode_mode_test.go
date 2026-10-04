package singlefile

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// TestVariablesFollowJSONVersion sends an Int above 2^53 in an input object
// variable. It reaches the resolver intact in both modes, as a json.Number in
// the variables in the JSONv1 mode and as a jsontext.Value in the JSONv2 mode.
func TestVariablesFollowJSONVersion(t *testing.T) {
	const big = 9007199254740993

	for _, tc := range []struct {
		version graphql.JSONVersion
		wantVar any
	}{
		{graphql.JSONv1, json.Number("9007199254740993")},
		{graphql.JSONv2, jsontext.Value("9007199254740993")},
	} {
		t.Run(tc.version.String(), func(t *testing.T) {
			var gotVar, gotArg any
			resolver := &Stub{}
			resolver.QueryResolver.MapStringInterface = func(ctx context.Context, in map[string]any) (map[string]any, error) {
				gotVar = graphql.GetOperationContext(ctx).Variables["in"].(map[string]any)["b"]
				gotArg = *in["b"].(*int)
				return in, nil
			}

			srv := handler.New(NewExecutableSchema(Config{Resolvers: resolver}))
			srv.AddTransport(transport.POST{})
			srv.SetJSONVersion(tc.version)
			c := client.New(srv, client.JSONVersion(tc.version))

			var resp struct{ MapStringInterface struct{ A string } }
			c.MustPost(
				`query($in: MapStringInterfaceInput) { mapStringInterface(in: $in) { a } }`,
				&resp,
				client.Var("in", map[string]any{"a": "x", "b": big}),
			)
			require.Equal(t, "x", resp.MapStringInterface.A)
			require.Equal(t, tc.wantVar, gotVar)
			require.Equal(t, big, gotArg)
		})
	}
}
