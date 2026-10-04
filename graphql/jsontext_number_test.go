package graphql

import (
	"encoding/json"
	"encoding/json/jsontext"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The JSONv2 mode delivers numbers in variables as jsontext.Value where the
// JSONv1 mode delivers json.Number. The built-in scalars must handle both alike.

var numberScalars = map[string]func(any) (any, error){
	"Int":    func(v any) (any, error) { return UnmarshalInt(v) },
	"Int8":   func(v any) (any, error) { return UnmarshalInt8(v) },
	"Int16":  func(v any) (any, error) { return UnmarshalInt16(v) },
	"Int32":  func(v any) (any, error) { return UnmarshalInt32(v) },
	"Int64":  func(v any) (any, error) { return UnmarshalInt64(v) },
	"Uint":   func(v any) (any, error) { return UnmarshalUint(v) },
	"Uint8":  func(v any) (any, error) { return UnmarshalUint8(v) },
	"Uint16": func(v any) (any, error) { return UnmarshalUint16(v) },
	"Uint32": func(v any) (any, error) { return UnmarshalUint32(v) },
	"Uint64": func(v any) (any, error) { return UnmarshalUint64(v) },
	"Float":  func(v any) (any, error) { return UnmarshalFloat(v) },
	"String": func(v any) (any, error) { return UnmarshalString(v) },
	"ID":     func(v any) (any, error) { return UnmarshalID(v) },
	"IntID":  func(v any) (any, error) { return UnmarshalIntID(v) },
	"UintID": func(v any) (any, error) { return UnmarshalUintID(v) },
}

func TestScalarsAcceptJSONTextNumbers(t *testing.T) {
	numbers := []string{
		"0", "123", "-1", "127", "128", "-129", "255", "256", "65536",
		"2147483648", "-2147483649", "4294967296", "9223372036854775807",
		"9223372036854775808", "18446744073709551616", "1.5", "1e3", "-0",
	}
	for name, unmarshal := range numberScalars {
		t.Run(name, func(t *testing.T) {
			for _, n := range numbers {
				want, wantErr := unmarshal(json.Number(n))
				got, gotErr := unmarshal(jsontext.Value(n))
				assert.Equal(t, want, got, n)
				if wantErr == nil {
					assert.NoError(t, gotErr, n)
				} else {
					assert.EqualError(t, gotErr, wantErr.Error(), n)
				}
			}
		})
	}
}

func TestScalarsRejectJSONTextNonNumbers(t *testing.T) {
	for name, unmarshal := range numberScalars {
		t.Run(name, func(t *testing.T) {
			for _, v := range []string{`"12"`, `true`, `null`, `[1]`, `{}`} {
				_, err := unmarshal(jsontext.Value(v))
				require.Error(t, err, v)
				assert.Contains(t, err.Error(), "jsontext.Value is not", v)
			}
		})
	}
}

func TestCoerceListJSONTextValues(t *testing.T) {
	v := jsontext.Value("12")
	assert.Equal(t, []any{v}, CoerceList([]jsontext.Value{v}))
	assert.Equal(t, []any{v}, CoerceList(v))
}
