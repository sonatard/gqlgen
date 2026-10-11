package returnpointersinunmarshalinput

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
)

// With return_pointers_in_unmarshalinput the generated helper returns *T, and the caller
// sees that in its signature rather than discovering it through a failed lookup. Note
// that the option changes only the unmarshaler return: the resolver argument shape still
// follows schema nullability.
func TestGeneratedTypedUnmarshalerReturnsPointer(t *testing.T) {
	// No fields and a null input yield a pointer to the zero value.
	objs := []any{map[string]any{"name": "bob"}, map[string]any{}, nil}
	var (
		called bool
		got    []*SearchFilters
		gotErr []error
	)

	resolvers := &Stub{}
	resolvers.QueryResolver.Search = func(ctx context.Context, filters SearchFilters) (string, error) {
		called = true
		for _, obj := range objs {
			res, err := UnmarshalSearchFilters(ctx, obj)
			got = append(got, res)
			gotErr = append(gotErr, err)
		}
		return "ok", nil
	}

	srv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolvers}))

	var resp struct{ Search string }
	require.NoError(t, client.New(srv).Post(`query { search(filters: {name: "x"}) }`, &resp))

	require.True(t, called, "the resolver never ran, so nothing was exercised")
	name := "bob"
	for i, want := range []SearchFilters{{Name: &name}, {}, {}} {
		require.NoError(t, gotErr[i])
		require.NotNil(t, got[i], "the unmarshaler must never return a nil pointer")
		assert.Equal(t, want, *got[i])
	}
}
