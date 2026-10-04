package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

func TestServerSetJSONVersion(t *testing.T) {
	srv := testserver.New()
	srv.AddTransport(transport.GET{})
	var got graphql.JSONVersion
	srv.AroundOperations(
		func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			got = graphql.GetJSONVersion(ctx)
			return next(ctx)
		},
	)

	query := func() graphql.JSONVersion {
		resp := get(srv, "/foo?query={name}")
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		return got
	}

	assert.Equal(t, graphql.JSONv1, query())
	srv.SetJSONVersion(graphql.JSONv2)
	assert.Equal(t, graphql.JSONv2, query())
}
