package execbehavior

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefer(t *testing.T) {
	resolvers := &Stub{}
	resolvers.QueryResolver.DeferItem = func(ctx context.Context) (*DeferItem, error) {
		return &DeferItem{ID: 1}, nil
	}
	resolvers.QueryResolver.DeferItems = func(ctx context.Context) ([]*DeferItem, error) {
		return []*DeferItem{{ID: 1}, {ID: 2}}, nil
	}
	resolvers.DeferItemResolver.Slow = func(ctx context.Context, obj *DeferItem) (string, error) {
		return "slow", nil
	}
	resolvers.DeferItemResolver.Failing = func(ctx context.Context, obj *DeferItem) (*string, error) {
		return nil, errors.New("failing field failed")
	}
	resolvers.DeferItemResolver.NonNullFailing = func(ctx context.Context, obj *DeferItem) (string, error) {
		return "", errors.New("non-null field failed")
	}
	es := NewExecutableSchema(Config{Resolvers: resolvers})

	t.Run("without defer", func(t *testing.T) {
		got := execute(t, es, `{ deferItem { id slow } }`)
		require.JSONEq(t, `[{"data":{"deferItem":{"id":1,"slow":"slow"}}}]`, got)
	})

	// A deferred field is sent as null in the first response, and with its value in the
	// response of its label.
	t.Run("deferred field", func(t *testing.T) {
		got := execute(t, es, `{ deferItem { id ... @defer(label: "later") { slow } } }`)
		require.JSONEq(t, `[
			{"data":{"deferItem":{"id":1,"slow":null}},"hasNext":true},
			{"data":{"slow":"slow"},"label":"later","path":["deferItem"]}
		]`, got)
	})

	t.Run("defer with if false", func(t *testing.T) {
		got := execute(t, es, `{ deferItem { id ... @defer(label: "later", if: false) { slow } } }`)
		require.JSONEq(t, `[{"data":{"deferItem":{"id":1,"slow":"slow"}}}]`, got)
	})

	t.Run("fields that are not resolved concurrently are not deferred", func(t *testing.T) {
		got := execute(t, es, `{ deferItem { ... @defer(label: "later") { id } slow } }`)
		require.JSONEq(t, `[{"data":{"deferItem":{"id":1,"slow":"slow"}}}]`, got)
	})

	// The errors of the deferred fields of one object are collected together, and each
	// deferred response takes those collected when its label completes. Which response
	// an error goes with thus depends on the order in which the labels complete, not on
	// its label. This records that the error is sent once, with one of them.
	t.Run("two labels", func(t *testing.T) {
		got := execute(t, es, `{ deferItem {
			id
			... @defer(label: "a") { slow }
			... @defer(label: "b") { failing }
		} }`)
		var responses []map[string]any
		require.NoError(t, json.Unmarshal([]byte(got), &responses))
		var errs []any
		for _, resp := range responses[1:] {
			if e, ok := resp["errors"].([]any); ok {
				errs = append(errs, e...)
				delete(resp, "errors")
			}
		}
		require.Equal(t, []any{map[string]any{
			"message": "failing field failed",
			"path":    []any{"deferItem", "failing"},
		}}, errs)
		rest, err := json.Marshal(responses)
		require.NoError(t, err)
		require.JSONEq(t, `[
			{"data":{"deferItem":{"id":1,"slow":null,"failing":null}},"hasNext":true},
			{"data":{"slow":"slow"},"label":"a","path":["deferItem"]},
			{"data":{"failing":null},"label":"b","path":["deferItem"]}
		]`, string(rest))
	})

	t.Run("a null non-null field nulls its deferred group", func(t *testing.T) {
		got := execute(
			t,
			es,
			`{ deferItem { id ... @defer(label: "later") { slow nonNullFailing } } }`,
		)
		require.JSONEq(t, `[
			{"data":{"deferItem":{"id":1,"slow":null,"nonNullFailing":null}},"hasNext":true},
			{"data":null,"label":"later","path":["deferItem"],
				"errors":[{"message":"non-null field failed","path":["deferItem","nonNullFailing"]}]}
		]`, got)
	})

	t.Run("deferred fields of list elements", func(t *testing.T) {
		got := execute(t, es, `{ deferItems { id ... @defer(label: "later") { slow } } }`)
		require.JSONEq(t, `[
			{"data":{"deferItems":[{"id":1,"slow":null},{"id":2,"slow":null}]},"hasNext":true},
			{"data":{"slow":"slow"},"label":"later","path":["deferItems",0]},
			{"data":{"slow":"slow"},"label":"later","path":["deferItems",1]}
		]`, got)
	})
}
