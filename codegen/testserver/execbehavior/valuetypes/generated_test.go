//go:generate ../../bothmodes.sh gqlgen.yml -stub stub.go

package valuetypes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestValueTypes(t *testing.T) {
	note := "noted"
	items := []Item{
		{
			Name:      "a",
			Note:      &note,
			Level:     LevelHigh,
			Levels:    []Level{LevelLow, LevelHigh},
			Tags:      []string{"x"},
			Owner:     &Owner{Name: "o"},
			MainOwner: Owner{Name: "main"},
		},
		{Name: "b", Level: LevelLow, Child: &Item{Name: "c", Level: LevelHigh}},
		// A Level that has no GraphQL value.
		{Name: "bad", Level: Level(99)},
	}
	resolvers := &Stub{}
	resolvers.QueryResolver.Items = func(ctx context.Context) ([]Item, error) {
		return items[:2], nil
	}
	resolvers.QueryResolver.Item = func(ctx context.Context, name string) (*Item, error) {
		for i := range items {
			if items[i].Name == name {
				return &items[i], nil
			}
		}
		return nil, nil
	}
	resolvers.QueryResolver.ItemValue = func(ctx context.Context, name string) (Item, error) {
		return Item{Name: name, Level: LevelLow}, nil
	}
	resolvers.QueryResolver.Echo = func(ctx context.Context, input []ItemInput) ([]Item, error) {
		res := make([]Item, len(input))
		for i, in := range input {
			res[i] = Item{Name: in.Name, Note: in.Note, Tags: in.Tags}
			if in.Level != nil {
				res[i].Level = *in.Level
			}
		}
		return res, nil
	}
	resolvers.QueryResolver.Levels = func(ctx context.Context, in []Level) ([]Level, error) {
		return append(in, LevelHigh), nil
	}
	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	for _, tc := range []struct{ name, query, want string }{
		{
			name:  "lists of values",
			query: `{ items { name note level levels tags child { name level } owner { name } mainOwner { name } } }`,
			want: `{"Data":{"items":[` +
				`{"name":"a","note":"noted","level":"HIGH","levels":["LOW","HIGH"],"tags":["x"],` +
				`"child":null,"owner":{"name":"o"},"mainOwner":{"name":"main"}},` +
				`{"name":"b","note":null,"level":"LOW","levels":[],"tags":null,` +
				`"child":{"name":"c","level":"HIGH"},"owner":null,"mainOwner":{"name":""}}` +
				`]},"Errors":null,"Extensions":null}`,
		},
		{
			name:  "nullable object",
			query: `{ item(name: "a") { name } missing: item(name: "z") { name } }`,
			want:  `{"Data":{"item":{"name":"a"},"missing":null},"Errors":null,"Extensions":null}`,
		},
		{
			name:  "object returned as a value",
			query: `{ itemValue(name: "v") { name level mainOwner { name } } }`,
			want: `{"Data":{"itemValue":{"name":"v","level":"LOW","mainOwner":{"name":""}}},` +
				`"Errors":null,"Extensions":null}`,
		},
		{
			name:  "list of inputs with an enum default",
			query: `{ echo(input: [{name: "d"}, {name: "e", level: HIGH, note: "n", tags: ["t"]}]) { name level note tags } }`,
			want: `{"Data":{"echo":[{"name":"d","level":"LOW","note":null,"tags":null},` +
				`{"name":"e","level":"HIGH","note":"n","tags":["t"]}]},"Errors":null,"Extensions":null}`,
		},
		{
			name:  "enum list",
			query: `{ levels(in: [LOW, HIGH]) }`,
			want:  `{"Data":{"levels":["LOW","HIGH","HIGH"]},"Errors":null,"Extensions":null}`,
		},
		{
			// The value is written as an empty string, without an error. This records
			// the current behavior.
			name:  "enum value without a GraphQL value",
			query: `{ item(name: "bad") { name level } }`,
			want:  `{"Data":{"item":{"name":"bad","level":""}},"Errors":null,"Extensions":null}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := c.RawPost(tc.query)
			require.NoError(t, err)
			b, err := json.Marshal(resp)
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(b))
		})
	}
}
