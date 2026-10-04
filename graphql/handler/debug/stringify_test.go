package debug

import (
	jsonv1 "encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStringifyMatchesMarshalIndent pins stringify to the
// encoding/json.MarshalIndent output it used to print.
func TestStringifyMatchesMarshalIndent(t *testing.T) {
	values := []any{
		nil,
		"<b> & </b>",
		1.5,
		map[string]any{},
		[]any{},
		map[string]any{
			"z": 1,
			"a": []any{1, map[string]any{}, []int{}, nil},
			"m": map[string]any{"k": "v"},
		},
		[]any{[]any{[]any{}}, map[string]any{"a": map[string]any{"b": []string{"c"}}}},
		struct {
			A int    `json:"a,omitempty"`
			B string `json:"b"`
			C []int  `json:"c"`
		}{},
		time.Second,
		jsonv1.Number("1.50"),
		make(chan int),
	}
	for i, v := range values {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			want := fmt.Sprint(v)
			if b, err := jsonv1.MarshalIndent(v, "  ", "  "); err == nil {
				want = string(b)
			}
			require.Equal(t, want, stringify(v))
		})
	}
}
