package nopanichandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
)

func TestSubscription(t *testing.T) {
	resolvers := &Stub{}
	resolvers.SubscriptionResolver.Count = func(ctx context.Context) (<-chan int, error) {
		ch := make(chan int, 2)
		ch <- 1
		ch <- 2
		close(ch)
		return ch, nil
	}
	resolvers.SubscriptionResolver.Failing = func(ctx context.Context) (<-chan int, error) {
		return nil, errors.New("failing stream")
	}
	resolvers.SubscriptionResolver.Panicking = func(ctx context.Context) (<-chan *int, error) {
		panic("subscription panicked")
	}
	es := NewExecutableSchema(Config{Resolvers: resolvers})

	for _, tc := range []struct{ query, want string }{
		{`subscription { count }`, `[{"data":{"count":1}},{"data":{"count":2}}]`},
		{
			`subscription { failing }`,
			`[{"errors":[{"message":"failing stream","path":["failing"]}],"data":null}]`,
		},
		{`subscription { panicking }`, `"panic: subscription panicked"`},
		{
			`subscription { count @skip(if: true) }`,
			`[{"errors":[{"message":"must subscribe to exactly one stream"}],"data":null}]`,
		},
	} {
		require.JSONEq(t, tc.want, subscribe(t, es, tc.query), tc.query)
	}
}

// subscribe runs the subscription query on es and returns its responses as a JSON array,
// without the locations of errors, or the value of a panic as a string.
func subscribe(t *testing.T, es graphql.ExecutableSchema, query string) (res string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			b, err := json.Marshal(fmt.Sprint("panic: ", r))
			require.NoError(t, err)
			res = string(b)
		}
	}()

	exec := executor.New(es)
	ctx := graphql.StartOperationTrace(context.Background())
	opCtx, errs := exec.CreateOperationContext(ctx, &graphql.RawParams{Query: query})
	require.Empty(t, errs)
	handler, ctx := exec.DispatchOperation(ctx, opCtx)

	// The handler reuses the buffer of the data, so each response is encoded before the
	// next one is read.
	var responses []json.RawMessage
	for range 100 {
		resp := handler(ctx)
		if resp == nil {
			break
		}
		for _, err := range resp.Errors {
			err.Locations = nil
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)
		responses = append(responses, b)
	}
	b, err := json.Marshal(responses)
	require.NoError(t, err)
	return string(b)
}
