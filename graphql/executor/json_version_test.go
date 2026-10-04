package executor_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/executor/testexecutor"
)

// jsonV2Schema stands in for a schema generated with json.version: v2.
type jsonV2Schema struct {
	graphql.ExecutableSchema
}

func (jsonV2Schema) JSONVersion() graphql.JSONVersion { return graphql.JSONv2 }

func TestJSONVersion(t *testing.T) {
	operationVersion := func(t *testing.T, exec *executor.Executor) graphql.JSONVersion {
		t.Helper()
		ctx := graphql.StartOperationTrace(context.Background())
		opCtx, errs := exec.CreateOperationContext(ctx, &graphql.RawParams{Query: "{name}"})
		require.Empty(t, errs)
		require.Equal(t, opCtx.JSONVersion,
			graphql.GetJSONVersion(graphql.WithOperationContext(ctx, opCtx)))
		return opCtx.JSONVersion
	}

	t.Run("defaults to v1", func(t *testing.T) {
		exec := executor.New(testexecutor.New().Schema())
		assert.Equal(t, graphql.JSONv1, exec.JSONVersion())
		assert.Equal(t, graphql.JSONv1, operationVersion(t, exec))
	})

	t.Run("generated schema sets the default", func(t *testing.T) {
		exec := executor.New(jsonV2Schema{testexecutor.New().Schema()})
		assert.Equal(t, graphql.JSONv2, exec.JSONVersion())
		assert.Equal(t, graphql.JSONv2, operationVersion(t, exec))
	})

	t.Run("SetJSONVersion overrides the generated schema", func(t *testing.T) {
		exec := executor.New(jsonV2Schema{testexecutor.New().Schema()})
		exec.SetJSONVersion(graphql.JSONv1)
		assert.Equal(t, graphql.JSONv1, exec.JSONVersion())
		assert.Equal(t, graphql.JSONv1, operationVersion(t, exec))
	})

	t.Run("no operation in the context", func(t *testing.T) {
		assert.Equal(t, graphql.JSONv1, graphql.GetJSONVersion(context.Background()))
	})
}

func TestJSONVersionString(t *testing.T) {
	assert.Equal(t, "v1", graphql.JSONv1.String())
	assert.Equal(t, "v2", graphql.JSONv2.String())
	assert.Equal(t, "JSONVersion(7)", graphql.JSONVersion(7).String())
}
