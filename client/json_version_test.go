package client_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/testserver"
	"github.com/99designs/gqlgen/graphql/handler/transport"
)

// encoding/json writes a nil slice as null and accepts duplicate names;
// encoding/json/v2 writes [] and rejects them. The tests below use these
// differences to tell which package the client uses.

func TestClientJSONVersion(t *testing.T) {
	var gotBody string
	respBody := `{"data":{"name":"bob"},"extensions":{"k":"v"}}`
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		gotBody = string(b)
		_, _ = io.WriteString(w, respBody)
	})
	list := client.Var("list", []string(nil))

	for _, tc := range []struct {
		name     string
		client   *client.Client
		opts     []client.Option
		wantBody string
	}{
		{"default", client.New(h), nil, `{"query":"{ name }","variables":{"list":null}}`},
		{
			"v2 for the client",
			client.New(h, client.JSONVersion(graphql.JSONv2)),
			nil,
			`{"query":"{ name }","variables":{"list":[]}}`,
		},
		{
			"v2 for a request",
			client.New(h),
			[]client.Option{client.JSONVersion(graphql.JSONv2)},
			`{"query":"{ name }","variables":{"list":[]}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			respBody = `{"data":{"name":"bob"},"extensions":{"k":"v"}}`
			var resp struct{ Name string }
			require.NoError(t, tc.client.Post("{ name }", &resp, append(tc.opts, list)...))
			assert.JSONEq(t, tc.wantBody, gotBody)
			assert.Equal(t, "bob", resp.Name)

			raw, err := tc.client.RawPost("{ name }", tc.opts...)
			require.NoError(t, err)
			assert.Equal(t, map[string]any{"name": "bob"}, raw.Data)
			assert.Equal(t, map[string]any{"k": "v"}, raw.Extensions)
		})
	}

	t.Run("v2 rejects duplicate names in the response", func(t *testing.T) {
		respBody = `{"data":{"name":"a","name":"b"}}`
		var resp struct{ Name string }

		require.NoError(t, client.New(h).Post("{ name }", &resp))
		assert.Equal(t, "b", resp.Name)

		err := client.New(h, client.JSONVersion(graphql.JSONv2)).Post("{ name }", &resp)
		require.ErrorContains(t, err, "decode:")
	})

	t.Run("WithFiles writes the operations with the selected package", func(t *testing.T) {
		respBody = `{"data":{"name":"bob"}}`
		c := client.New(h, client.JSONVersion(graphql.JSONv2))
		var resp struct{ Name string }
		require.NoError(t, c.Post("{ name }", &resp, list, client.WithFiles()))
		assert.Contains(t, gotBody, `"variables":{"list":[]}`)
	})
}

func TestClientJSONVersionSubscriptions(t *testing.T) {
	for _, version := range []graphql.JSONVersion{graphql.JSONv1, graphql.JSONv2} {
		t.Run(version.String(), func(t *testing.T) {
			t.Run("websocket", func(t *testing.T) {
				h := testserver.New()
				h.AddTransport(transport.Websocket{KeepAlivePingInterval: time.Second})
				c := client.New(h, client.JSONVersion(version))

				sub := c.Websocket(`subscription { name }`)
				defer sub.Close()

				defer sendUntilDone(h)()
				var resp struct{ Name string }
				require.NoError(t, sub.Next(&resp))
				assert.Equal(t, "test", resp.Name)
			})

			t.Run("sse", func(t *testing.T) {
				h := testserver.New()
				h.AddTransport(transport.SSE{})
				c := client.New(h, client.JSONVersion(version))

				// SSE returns once the server has written the whole response.
				go func() {
					h.SendNextSubscriptionMessage()
					h.SendCompleteSubscriptionMessage()
				}()
				sub := c.SSE(context.Background(), `subscription { name }`)
				defer sub.Close()

				var resp client.SSEResponse
				require.NoError(t, sub.Next(&resp))
				assert.Equal(t, map[string]any{"name": "test"}, resp.Data)
			})
		})
	}
}

// sendUntilDone sends subscription messages until the returned function is
// called, since the subscription may not have started when the client
// returns.
func sendUntilDone(h *testserver.TestServer) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			default:
				h.SendNextSubscriptionMessage()
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}
