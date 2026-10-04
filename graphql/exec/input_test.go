package exec

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/99designs/gqlgen/graphql"
)

type testEC struct{}

type point struct {
	X, Y int
	Note *string
}

func unmarshalInt(_ context.Context, _ *testEC, v any) (int, error) {
	i, ok := v.(int)
	if !ok {
		return 0, errors.New("not an int")
	}
	return i, nil
}

func unmarshalString(_ context.Context, _ *testEC, v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("not a string")
	}
	return &s, nil
}

var (
	inInt    = InOf(unmarshalInt)
	inString = InOf(unmarshalString)
)

func pointInput() *Input[*testEC] {
	return &Input[*testEC]{
		Name: "Point",
		Fields: []InputField[*testEC]{
			{Name: "x", Type: inInt, Set: func(it, v any) { it.(*point).X, _ = v.(int) }},
			{
				Name:    "y",
				Default: 7,
				Type:    inInt,
				Set:     func(it, v any) { it.(*point).Y, _ = v.(int) },
			},
			{
				Name: "note",
				Type: inString,
				Set:  func(it, v any) { it.(*point).Note, _ = v.(*string) },
			},
		},
	}
}

func TestInputUnmarshal(t *testing.T) {
	ctx := context.Background()
	in := pointInput()

	p, err := InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, point{X: 1, Y: 7}, p)

	p, err = InputValue[*testEC, point](ctx, nil, in, nil)
	require.NoError(t, err)
	require.Equal(t, point{}, p)

	pp, err := InputPointer[*testEC, point](ctx, nil, in, map[string]any{"x": 2, "y": 3})
	require.NoError(t, err)
	require.Equal(t, &point{X: 2, Y: 3}, pp)

	_, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": "one"})
	require.EqualError(t, err, "not an int")

	p, err = InputFunc[*testEC, point](in)(ctx, nil, map[string]any{"x": 4})
	require.NoError(t, err)
	require.Equal(t, point{X: 4, Y: 7}, p)
}

func TestInputFieldDirectives(t *testing.T) {
	ctx := graphql.WithPathContext(context.Background(), graphql.NewPathWithField("arg"))
	in := pointInput()

	in.Fields[2].Directives = returning(nil, nil)
	in.Fields[2].NilOK = true
	p, err := InputValue[*testEC, point](ctx, nil, in, map[string]any{"note": "dropped"})
	require.NoError(t, err)
	require.Nil(t, p.Note)

	in.Fields[0].Directives = returning(nil, nil)
	_, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.EqualError(t, err, "input: arg.x unexpected type <nil> from directive, should be int")

	in.Fields[0].Directives = returning("one", nil)
	_, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.EqualError(t, err, "input: arg.x unexpected type string from directive, should be int")

	in.Fields[0].Directives = returning(5, nil)
	p, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, 5, p.X)
}

func TestInputObjectDirectives(t *testing.T) {
	ctx := context.Background()
	in := pointInput()
	replace := func(v any) []Directive[*testEC] {
		def := &DirectiveDef[*testEC]{
			Name: "replace",
			Call: func(ctx context.Context, ec *testEC, obj any, next graphql.Resolver, args map[string]any) (any, error) {
				require.IsType(t, map[string]any{}, obj)
				got, err := next(ctx)
				if err != nil || v == nil {
					return got, err
				}
				return v, nil
			},
		}
		return Dirs(def.With(nil))
	}

	in.Directives = replace(nil)
	p, err := InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, point{X: 1, Y: 7}, p)
	pp, err := InputPointer[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, &point{X: 1, Y: 7}, pp)

	in.Directives = replace(point{X: 9})
	p, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Equal(t, point{X: 9}, p)

	// With pointers a directive may replace the input with nil.
	in.Directives = replace((*point)(nil))
	pp, err = InputPointer[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.NoError(t, err)
	require.Nil(t, pp)

	in.Directives = replace("wrong")
	_, err = InputValue[*testEC, point](ctx, nil, in, map[string]any{"x": 1})
	require.EqualError(
		t,
		err,
		"input: unexpected type string from INPUT_OBJECT directive, should be exec.point",
	)
}

func TestInputMap(t *testing.T) {
	in := &Input[*testEC]{
		Name:  "Map",
		IsMap: true,
		Fields: []InputField[*testEC]{
			{
				Name: "x",
				Type: inInt,
				Set:  func(it, v any) { x, _ := v.(int); (*it.(*map[string]any))["x"] = x },
			},
		},
	}
	m, err := InputValue[*testEC, map[string]any](
		context.Background(),
		nil,
		in,
		map[string]any{"x": 1, "ignored": true},
	)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"x": 1}, m)
}

func TestInputFieldSetByResolver(t *testing.T) {
	in := &Input[*testEC]{
		Name: "Point",
		Fields: []InputField[*testEC]{
			{
				Name: "x",
				Type: inInt,
				SetWith: func(ctx context.Context, ec *testEC, it, v any) error {
					x, _ := v.(int)
					if x < 0 {
						return errors.New("negative")
					}
					it.(*point).X = x * 10
					return nil
				},
			},
		},
	}
	p, err := InputValue[*testEC, point](context.Background(), nil, in, map[string]any{"x": 2})
	require.NoError(t, err)
	require.Equal(t, 20, p.X)

	_, err = InputValue[*testEC, point](context.Background(), nil, in, map[string]any{"x": -1})
	require.EqualError(t, err, "negative")
}

// returning returns directives that call next and then return v instead of its result.
// onCall, when set, is called with the object the directive receives.
func returning(v any, onCall func(obj any)) []Directive[*testEC] {
	def := &DirectiveDef[*testEC]{
		Name: "returning",
		Call: func(ctx context.Context, ec *testEC, obj any, next graphql.Resolver, args map[string]any) (any, error) {
			if onCall != nil {
				onCall(obj)
			}
			if _, err := next(ctx); err != nil {
				return nil, err
			}
			return v, nil
		},
	}
	return Dirs(def.With(nil))
}

func TestTypeString(t *testing.T) {
	require.Equal(t, "int", typeString(reflect.TypeFor[int]()))
	require.Equal(t, "*string", typeString(reflect.TypeFor[*string]()))
	require.Equal(
		t,
		"[]*github.com/99designs/gqlgen/graphql/exec.point",
		typeString(reflect.TypeFor[[]*point]()),
	)
	require.Equal(t, "map[string]any", typeString(reflect.TypeFor[map[string]any]()))
	require.Equal(
		t,
		"github.com/99designs/gqlgen/graphql.Marshaler",
		typeString(reflect.TypeFor[graphql.Marshaler]()),
	)
}
