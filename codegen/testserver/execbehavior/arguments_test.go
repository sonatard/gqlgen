package execbehavior

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

func TestArguments(t *testing.T) {
	resolvers := &Stub{}
	resolvers.QueryResolver.ArgProbe = func(ctx context.Context, strict *Strict, withDefault *int, plain *string) (*string, error) {
		describe := func(v any) string {
			switch v := v.(type) {
			case *Strict:
				if v != nil {
					return string(*v)
				}
			case *int:
				if v != nil {
					return strconv.Itoa(*v)
				}
			case *string:
				if v != nil {
					return *v
				}
			}
			return "nil"
		}
		s := describe(strict) + "," + describe(withDefault) + "," + describe(plain)
		return &s, nil
	}
	resolvers.QueryResolver.ArgObject = func(ctx context.Context) (*ArgObject, error) {
		return &ArgObject{Ok: "ok"}, nil
	}
	resolvers.ArgObjectResolver.Echo = func(ctx context.Context, obj *ArgObject, strict Strict) (*string, error) {
		s := string(strict)
		return &s, nil
	}
	resolvers.QueryResolver.WrongTypeArg = func(ctx context.Context, value *string) (*string, error) {
		return value, nil
	}
	called := false
	srv := newServer(resolvers, DirectiveRoot{
		ReturnValue: func(ctx context.Context, obj any, next graphql.Resolver, kind string) (any, error) {
			called = true
			return next(ctx)
		},
	})

	t.Run("omitted, default and explicit null", func(t *testing.T) {
		got := post(t, srv, `{
			omitted: argProbe
			given: argProbe(strict: "s", withDefault: 1, plain: "p")
			nulls: argProbe(strict: null, withDefault: null, plain: null)
		}`)
		require.JSONEq(
			t,
			`{"data":{"omitted":"nil,7,nil","given":"s,1,p","nulls":"nil,nil,nil"}}`,
			got,
		)
	})

	t.Run("invalid literal", func(t *testing.T) {
		got := post(t, srv, `{ argProbe(strict: "bad") }`)
		require.JSONEq(t, `{"data":{"argProbe":null},"errors":[
			{"message":"strict rejects this value","path":["argProbe","strict"]}]}`, got)
	})

	t.Run("invalid variable", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"query":     `query($s: Strict) { argProbe(strict: $s) }`,
			"variables": map[string]any{"s": "bad"},
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		// KNOWN BUG: an invalid variable fails only the field that uses it, with an error
		// without locations.
		// Expected: a request error and no data, because the spec coerces the values of
		// the variables before it executes the operation, and fails the request when one
		// cannot be coerced.
		require.JSONEq(t, `{"data":{"argProbe":null},"errors":[
			{"message":"strict rejects this value","path":["argProbe","strict"]}]}`,
			w.Body.String())
	})

	t.Run("invalid argument of a nested field", func(t *testing.T) {
		got := post(t, srv, `{ argObject { ok echo(strict: "bad") good: echo(strict: "good") } }`)
		require.JSONEq(t, `{"data":{"argObject":{"ok":"ok","echo":null,"good":"good"}},"errors":[
			{"message":"strict rejects this value","path":["argObject","echo","strict"]}]}`, got)
	})

	t.Run("directives of an omitted argument are not called", func(t *testing.T) {
		called = false
		got := post(t, srv, `{ wrongTypeArg }`)
		require.JSONEq(t, `{"data":{"wrongTypeArg":null}}`, got)
		require.False(t, called)
	})
}
