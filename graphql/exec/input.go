package exec

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/99designs/gqlgen/graphql"
)

// Unmarshal converts a raw input value into V. Generated code passes its unmarshal
// functions.
type Unmarshal[EC, V any] func(ctx context.Context, ec EC, v any) (V, error)

// In unmarshals raw input values into one Go type, returned as any. Generated code
// declares one per type that arguments and input fields take, with InOf.
type In[EC any] struct {
	unmarshal func(ctx context.Context, ec EC, v any) (any, error)
	accept    func(v any) bool
	zero      any
	name      string
}

// InOf returns the In of the values that u unmarshals.
func InOf[EC, V any](u Unmarshal[EC, V]) *In[EC] {
	var zero V
	return &In[EC]{
		unmarshal: func(ctx context.Context, ec EC, v any) (any, error) {
			return u(ctx, ec, v)
		},
		accept: func(v any) bool {
			_, ok := v.(V)
			return ok
		},
		zero: zero,
		name: typeString(reflect.TypeFor[V]()),
	}
}

// Input describes how a GraphQL input object is unmarshaled into its Go type.
//
// Generated code declares one Input per input object and assigns Fields in an init
// function, because the unmarshal functions the fields refer to may refer back to the
// input. The Go value is created by InputValue or InputPointer, and passed to the fields
// as a pointer in an any, so that one instantiation of the runtime serves every input.
type Input[EC any] struct {
	Name string
	// Fields are processed in this order, which is the order of the schema definition.
	Fields []InputField[EC]
	// GoType is the Go type of the input as the generated package writes it. Errors
	// name it.
	GoType string
	// IsMap is set when the Go type is map[string]any. The map is made before the
	// fields are set.
	IsMap bool
	// Directives are the INPUT_OBJECT directives, run after all fields are set. They
	// receive the raw input object.
	Directives []Directive[EC]
}

// InputField describes one field of an input object.
type InputField[EC any] struct {
	Name string
	// Default is the default value from the schema, applied when the field is absent.
	// A nil Default means the field has no default.
	Default any
	// Directives are the directives declared on the field. They receive the raw input
	// object.
	Directives []Directive[EC]
	// NilOK is set when a directive may return nil for the field, which then sets the
	// field to its zero value.
	NilOK bool
	// Type unmarshals the field.
	Type *In[EC]
	// Set stores v, a value of the field's Go type or nil for its zero value, in it, a
	// pointer to the input's Go type.
	Set func(it, v any)
	// SetWith is set instead of Set for fields that a resolver sets.
	SetWith func(ctx context.Context, ec EC, it, v any) error
}

// InputValue unmarshals the raw input object obj into T, the Go type of in.
func InputValue[EC, T any](ctx context.Context, ec EC, in *Input[EC], obj any) (T, error) {
	var it T
	res, replaced, err := in.unmarshal(ctx, ec, obj, &it, func() any { return it })
	if err != nil || !replaced {
		return it, err
	}
	if data, ok := res.(T); ok {
		return data, nil
	}
	return it, graphql.ErrorOnPath(ctx, fmt.Errorf(
		"unexpected type %T from INPUT_OBJECT directive, should be %s",
		res,
		in.goType(reflect.TypeFor[T]()),
	))
}

// InputPointer is InputValue for inputs generated with
// return_pointers_in_unmarshalinput. An INPUT_OBJECT directive may replace the result,
// including with nil.
func InputPointer[EC, T any](ctx context.Context, ec EC, in *Input[EC], obj any) (*T, error) {
	it := new(T)
	res, replaced, err := in.unmarshal(ctx, ec, obj, it, func() any { return it })
	if err != nil || !replaced {
		return it, err
	}
	if data, ok := res.(*T); ok {
		return data, nil
	}
	return it, graphql.ErrorOnPath(ctx, fmt.Errorf(
		"unexpected type %T from INPUT_OBJECT directive, should be *%s",
		res,
		in.goType(reflect.TypeFor[T]()),
	))
}

// InputFunc returns InputValue as a function of the raw input, for the generated index
// of input unmarshalers.
func InputFunc[EC, T any](in *Input[EC]) func(ctx context.Context, ec EC, obj any) (T, error) {
	return func(ctx context.Context, ec EC, obj any) (T, error) {
		return InputValue[EC, T](ctx, ec, in, obj)
	}
}

// InputPointerFunc is InputFunc for InputPointer.
func InputPointerFunc[EC, T any](
	in *Input[EC],
) func(ctx context.Context, ec EC, obj any) (*T, error) {
	return func(ctx context.Context, ec EC, obj any) (*T, error) {
		return InputPointer[EC, T](ctx, ec, in, obj)
	}
}

// unmarshal sets the fields of it, a pointer to the Go type of in, from the raw input
// object obj. When in has INPUT_OBJECT directives it returns their result, which
// replaces the input, and replaced is true; the directives receive current().
func (in *Input[EC]) unmarshal(
	ctx context.Context,
	ec EC,
	obj, it any,
	current func() any,
) (res any, replaced bool, err error) {
	if obj == nil {
		return nil, false, nil
	}

	src := obj.(map[string]any)
	asMap := make(map[string]any, len(src))
	maps.Copy(asMap, src)
	for i := range in.Fields {
		f := &in.Fields[i]
		if f.Default == nil {
			continue
		}
		if _, present := asMap[f.Name]; !present {
			asMap[f.Name] = f.Default
		}
	}

	if in.IsMap {
		*it.(*map[string]any) = make(map[string]any, len(asMap))
	}

	for i := range in.Fields {
		f := &in.Fields[i]
		v, ok := asMap[f.Name]
		if !ok {
			continue
		}
		if err := f.unmarshal(ctx, ec, obj, it, v); err != nil {
			return nil, false, err
		}
	}

	if len(in.Directives) == 0 {
		return nil, false, nil
	}

	next := func(ctx context.Context) (any, error) { return current(), nil }
	res, err = Chain(ec, asMap, in.Directives, next)(ctx)
	if err != nil {
		return nil, false, graphql.ErrorOnPath(ctx, err)
	}
	return res, true, nil
}

func (f *InputField[EC]) unmarshal(ctx context.Context, ec EC, obj, it, raw any) error {
	ctx = graphql.WithPathContext(ctx, graphql.NewPathWithField(f.Name))
	if len(f.Directives) == 0 {
		data, err := f.Type.unmarshal(ctx, ec, raw)
		if err != nil {
			return err
		}
		return f.store(ctx, ec, it, data)
	}

	next := func(ctx context.Context) (any, error) { return f.Type.unmarshal(ctx, ec, raw) }
	tmp, err := Chain(ec, obj, f.Directives, next)(ctx)
	if err != nil {
		return graphql.ErrorOnPath(ctx, err)
	}
	if f.Type.accept(tmp) || (tmp == nil && f.NilOK) {
		return f.store(ctx, ec, it, tmp)
	}
	return graphql.ErrorOnPath(ctx, fmt.Errorf(
		"unexpected type %T from directive, should be %s", tmp, f.Type.name))
}

func (f *InputField[EC]) store(ctx context.Context, ec EC, it, v any) error {
	if f.SetWith != nil {
		return f.SetWith(ctx, ec, it, v)
	}
	f.Set(it, v)
	return nil
}

// goType returns the name of the input's Go type t for errors.
func (in *Input[EC]) goType(t reflect.Type) string {
	if in.GoType != "" {
		return in.GoType
	}
	return t.String()
}

// typeString returns t as go/types writes it, with the full import path of named types,
// which is how the functions mode names types in errors.
func typeString(t reflect.Type) string {
	if t.Name() != "" {
		if t.PkgPath() == "" {
			return t.Name()
		}
		return t.PkgPath() + "." + t.Name()
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + typeString(t.Elem())
	case reflect.Slice:
		return "[]" + typeString(t.Elem())
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), typeString(t.Elem()))
	case reflect.Map:
		return "map[" + typeString(t.Key()) + "]" + typeString(t.Elem())
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return "any"
		}
	}
	return t.String()
}
