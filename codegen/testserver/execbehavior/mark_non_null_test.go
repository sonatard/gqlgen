package execbehavior

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

func TestMarkNonNull(t *testing.T) {
	var value, resolved *string
	var resolvedErr error
	resolvers := &Stub{}
	resolvers.QueryResolver.MarkedParent = func(ctx context.Context) (*MarkedParent, error) {
		name := "parent"
		return &MarkedParent{Name: &name, Child: &MarkedChild{Value: value}}, nil
	}
	resolvers.MarkedChildResolver.ResolvedValue = func(ctx context.Context, obj *MarkedChild) (*string, error) {
		return resolved, resolvedErr
	}
	resolvers.MarkedChildResolver.Node = func(ctx context.Context, obj *MarkedChild) (MarkedNode, error) {
		return nil, nil
	}
	marked := map[string]bool{}
	srv := newServer(resolvers, DirectiveRoot{})
	srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (any, error) {
		if marked[graphql.GetFieldContext(ctx).Field.Name] {
			graphql.MarkNonNull(ctx)
		}
		return next(ctx)
	})
	s := "s"

	tests := []struct {
		name        string
		marked      string
		value       *string
		resolved    *string
		resolvedErr error
		query       string
		want        string
	}{
		{
			name:  "unmarked null field stays null",
			value: nil,
			query: `{ markedParent { name child { value } } }`,
			want:  `{"data":{"markedParent":{"name":"parent","child":{"value":null}}}}`,
		},
		{
			name:   "marked field with a value",
			marked: "value",
			value:  &s,
			query:  `{ markedParent { name child { value } } }`,
			want:   `{"data":{"markedParent":{"name":"parent","child":{"value":"s"}}}}`,
		},
		{
			name:        "marked resolver field that fails nulls its parent",
			marked:      "resolvedValue",
			resolvedErr: errors.New("resolver failed"),
			query:       `{ markedParent { name child { resolvedValue } } }`,
			want: `{"data":{"markedParent":{"name":"parent","child":null}},"errors":[
				{"message":"resolver failed","path":["markedParent","child","resolvedValue"]}]}`,
		},
		{
			name:   "marked field resolving to a nil interface nulls its parent",
			marked: "node",
			query:  `{ markedParent { name child { node { id } } } }`,
			want: `{"data":{"markedParent":{"name":"parent","child":null}},"errors":[
				{"message":"must not be null","path":["markedParent","child","node"]}]}`,
		},
		{
			name:   "marked struct field holding a nil interface nulls its parent",
			marked: "plainNode",
			query:  `{ markedParent { name child { plainNode { id } } } }`,
			want: `{"data":{"markedParent":{"name":"parent","child":null}},"errors":[
				{"message":"must not be null","path":["markedParent","child","plainNode"]}]}`,
		},
		// The documentation of MarkNonNull says that a marked field resolving to nil
		// propagates the null. The executor only sees a nil interface as nil, so a nil
		// pointer, which reaches it as a non-nil any, is marshaled as null without an
		// error. These cases record that current behaviour.
		{
			name:   "marked struct field holding a nil pointer stays null",
			marked: "value",
			query:  `{ markedParent { name child { value } } }`,
			want:   `{"data":{"markedParent":{"name":"parent","child":{"value":null}}}}`,
		},
		{
			name:   "marked resolver field returning a nil pointer stays null",
			marked: "resolvedValue",
			query:  `{ markedParent { name child { resolvedValue } } }`,
			want:   `{"data":{"markedParent":{"name":"parent","child":{"resolvedValue":null}}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marked = map[string]bool{tt.marked: true}
			value, resolved, resolvedErr = tt.value, tt.resolved, tt.resolvedErr
			require.JSONEq(t, tt.want, post(t, srv, tt.query))
		})
	}
}
