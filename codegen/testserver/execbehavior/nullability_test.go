package execbehavior

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNonNullViolations(t *testing.T) {
	s := "s"
	resolvers := &Stub{}
	resolvers.QueryResolver.Nullability = func(ctx context.Context, valid bool) (*Nullability, error) {
		if valid {
			return &Nullability{
				Optional:       &s,
				Required:       &s,
				RequiredElems:  []*string{&s},
				RequiredList:   []*string{&s, nil},
				Nested:         [][]*string{{&s}},
				RequiredObject: &Dog{Name: "rex"},
			}, nil
		}
		return &Nullability{
			RequiredDogs:  []*Dog{{Name: "rex"}, nil},
			RequiredElems: []*string{&s, nil},
			Nested:        [][]*string{{&s}, {nil}},
		}, nil
	}
	srv := newServer(resolvers, DirectiveRoot{})
	const notAllowed = "the requested element is null which the schema does not allow"

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name: "valid values",
			query: `{ nullability(valid: true) {
				optional required requiredElems requiredList nested requiredObject { name }
			} }`,
			want: `{"data":{"nullability":{
				"optional":"s","required":"s","requiredElems":["s"],"requiredList":["s",null],
				"nested":[["s"]],"requiredObject":{"name":"rex"}
			}}}`,
		},
		{
			name:  "nil in a nullable field",
			query: `{ nullability(valid: false) { optional } }`,
			want:  `{"data":{"nullability":{"optional":null}}}`,
		},
		{
			name:  "nil in a non-null field nulls the parent",
			query: `{ nullability(valid: false) { optional required } }`,
			want: `{"data":{"nullability":null},"errors":[
				{"message":"` + notAllowed + `","path":["nullability","required"]}]}`,
		},
		// Lists of scalars marshal their elements without a path of their own, so the
		// error of a null element names the list, not the element.
		{
			name:  "nil element of a list of non-null elements nulls the list",
			query: `{ nullability(valid: false) { optional requiredElems } }`,
			want: `{"data":{"nullability":{"optional":null,"requiredElems":null}},"errors":[
				{"message":"` + notAllowed + `","path":["nullability","requiredElems"]}]}`,
		},
		{
			// A nil slice in a non-null list position is marshaled as an empty list.
			name:  "nil non-null list",
			query: `{ nullability(valid: false) { requiredList } }`,
			want:  `{"data":{"nullability":{"requiredList":[]}}}`,
		},
		{
			name:  "nil in an inner list nulls the outer list",
			query: `{ nullability(valid: false) { optional nested } }`,
			want: `{"data":{"nullability":{"optional":null,"nested":null}},"errors":[
				{"message":"` + notAllowed + `","path":["nullability","nested"]}]}`,
		},
		{
			name:  "nil element of a list of non-null objects names the element",
			query: `{ nullability(valid: false) { optional requiredDogs { name } } }`,
			want: `{"data":{"nullability":{"optional":null,"requiredDogs":null}},"errors":[
				{"message":"` + notAllowed + `","path":["nullability","requiredDogs",1]}]}`,
		},
		{
			name:  "nil non-null object nulls the parent",
			query: `{ nullability(valid: false) { optional requiredObject { name } } }`,
			want: `{"data":{"nullability":null},"errors":[
				{"message":"` + notAllowed + `","path":["nullability","requiredObject"]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.JSONEq(t, tt.want, post(t, srv, tt.query))
		})
	}
}
