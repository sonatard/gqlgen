package execbehavior

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFunctionScalars(t *testing.T) {
	resolvers := &Stub{}
	ptr := func(v string) *string {
		if v == "nil" {
			return nil
		}
		return &v
	}
	resolvers.QueryResolver.Maybe = func(ctx context.Context, v string) (*string, error) {
		return ptr(v), nil
	}
	resolvers.QueryResolver.MaybeNonNull = func(ctx context.Context, v string) (string, error) {
		return v, nil
	}
	resolvers.QueryResolver.CtxMaybe = func(ctx context.Context, v *string) (*string, error) {
		return v, nil
	}
	resolvers.QueryResolver.CtxMaybeNonNull = func(ctx context.Context, v string) (string, error) {
		return v, nil
	}
	srv := newServer(resolvers, DirectiveRoot{})

	nullError := `"errors":[{"message":"the requested element is null which the schema does not allow",`
	for _, tc := range []struct{ name, query, want string }{
		{"value", `{ maybe(v: "x") }`, `{"data":{"maybe":"x"}}`},
		{"marshaler returns null", `{ maybe(v: "") }`, `{"data":{"maybe":null}}`},
		{"nil pointer", `{ maybe(v: "nil") }`, `{"data":{"maybe":null}}`},
		{
			"marshaler returns null for a non-null field",
			`{ maybeNonNull(v: "") }`,
			`{"data":null,` + nullError + `"path":["maybeNonNull"]}]}`,
		},
		{"context marshaler", `{ ctxMaybe(v: "y") }`, `{"data":{"ctxMaybe":"ctxMaybe:y"}}`},
		{"null argument", `{ ctxMaybe(v: null) }`, `{"data":{"ctxMaybe":null}}`},
		{"absent argument", `{ ctxMaybe }`, `{"data":{"ctxMaybe":null}}`},
		{"context marshaler returns null", `{ ctxMaybe(v: "") }`, `{"data":{"ctxMaybe":null}}`},
		{
			// The error is reported, but the null does not propagate to the parent. This
			// records the current behavior.
			"context marshaler returns null for a non-null field",
			`{ ctxMaybeNonNull(v: "") }`,
			`{"data":{"ctxMaybeNonNull":null},` + nullError + `"path":["ctxMaybeNonNull"]}]}`,
		},
		{
			"context unmarshaler fails",
			`{ ctxMaybeNonNull(v: "bad") }`,
			`{"data":null,"errors":[{"message":"bad CtxMaybe","path":["ctxMaybeNonNull","v"]}]}`,
		},
		{
			"context unmarshaler fails for a pointer",
			`{ ctxMaybe(v: "bad") }`,
			`{"data":{"ctxMaybe":null},"errors":[{"message":"bad CtxMaybe","path":["ctxMaybe","v"]}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.JSONEq(t, tc.want, post(t, srv, tc.query))
		})
	}
}
