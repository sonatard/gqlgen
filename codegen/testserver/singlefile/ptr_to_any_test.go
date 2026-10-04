package singlefile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestPtrToAny(t *testing.T) {
	resolvers := &Stub{}

	srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers}))
	srv.AddTransport(transport.POST{})
	c := client.New(srv)

	var a any = `{"some":"thing"}`
	resolvers.QueryResolver.PtrToAnyContainer = func(ctx context.Context) (wrappedStruct *PtrToAnyContainer, e error) {
		ptrToAnyContainer := PtrToAnyContainer{
			PtrToAny: &a,
		}
		return &ptrToAnyContainer, nil
	}

	t.Run("binding to pointer to any", func(t *testing.T) {
		var resp struct {
			PtrToAnyContainer struct {
				Binding *any
			}
		}

		err := c.Post(`query { ptrToAnyContainer { binding }}`, &resp)
		require.NoError(t, err)

		require.Equal(t, &a, resp.PtrToAnyContainer.Binding)
	})
}

func TestAnyFollowsJSONVersion(t *testing.T) {
	resolvers := &Stub{}
	// encoding/json writes a nil slice as null, encoding/json/v2 as [].
	var a any = map[string]any{"list": []string(nil)}
	resolvers.QueryResolver.PtrToAnyContainer = func(ctx context.Context) (*PtrToAnyContainer, error) {
		return &PtrToAnyContainer{PtrToAny: &a}, nil
	}

	for _, tc := range []struct {
		version graphql.JSONVersion
		want    string
	}{
		{graphql.JSONv1, `{"data":{"ptrToAnyContainer":{"ptrToAny":{"list":null}}}}`},
		{graphql.JSONv2, `{"data":{"ptrToAnyContainer":{"ptrToAny":{"list":[]}}}}`},
	} {
		t.Run(tc.version.String(), func(t *testing.T) {
			srv := handler.New(NewExecutableSchema(Config{Resolvers: resolvers}))
			srv.AddTransport(transport.POST{})
			srv.SetJSONVersion(tc.version)

			body := `{"query":"{ ptrToAnyContainer { ptrToAny } }"}`
			r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, r)

			require.Equal(t, tc.want, w.Body.String())
		})
	}
}
