package exec

import (
	"context"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql"
)

// The functions in this file build the marshal and unmarshal functions of the most
// common type references, so that table mode generates a variable holding one of them
// instead of a function. Each one does exactly what the functions mode generates for
// the same type reference.

// null is the value of a null element. In a non-null position it also reports the
// error the functions mode reports, unless the field already has one.
func null(ctx context.Context, nonNull bool) graphql.Marshaler {
	if nonNull && !graphql.HasFieldError(ctx, graphql.GetFieldContext(ctx)) {
		graphql.AddErrorf(ctx, "the requested element is null which the schema does not allow")
	}
	return graphql.Null
}

// MarshalInterface marshals a value of an interface type with m, and nil as null.
func MarshalInterface[EC, V any](nonNull bool, m Marshal[EC, V]) Marshal[EC, V] {
	return func(ctx context.Context, ec EC, sel ast.SelectionSet, v V) graphql.Marshaler {
		if any(v) == nil {
			return null(ctx, nonNull)
		}
		return m(ctx, ec, sel, v)
	}
}

// MarshalSelf marshals a value that implements graphql.Marshaler.
func MarshalSelf[EC any, V graphql.Marshaler]() Marshal[EC, V] {
	return func(_ context.Context, _ EC, _ ast.SelectionSet, v V) graphql.Marshaler {
		return v
	}
}

// MarshalSelfPtr marshals a pointer that implements graphql.Marshaler, and nil as null.
func MarshalSelfPtr[EC, E any, PE interface {
	*E
	graphql.Marshaler
}](nonNull bool) Marshal[EC, *E] {
	return func(ctx context.Context, _ EC, _ ast.SelectionSet, v *E) graphql.Marshaler {
		if v == nil {
			return null(ctx, nonNull)
		}
		return PE(v)
	}
}

// MarshalFunc marshals a value with a scalar marshaler such as graphql.MarshalString.
func MarshalFunc[EC, V any](nonNull bool, f func(V) graphql.Marshaler) Marshal[EC, V] {
	return func(ctx context.Context, _ EC, _ ast.SelectionSet, v V) graphql.Marshaler {
		res := f(v)
		if res == graphql.Null {
			return null(ctx, nonNull)
		}
		return res
	}
}

// MarshalFuncPtr marshals a pointer with a scalar marshaler of its element, and nil as
// null.
func MarshalFuncPtr[EC, E any](nonNull bool, f func(E) graphql.Marshaler) Marshal[EC, *E] {
	return func(ctx context.Context, _ EC, _ ast.SelectionSet, v *E) graphql.Marshaler {
		if v == nil {
			return null(ctx, nonNull)
		}
		res := f(*v)
		if res == graphql.Null {
			return null(ctx, nonNull)
		}
		return res
	}
}

// ListOptions describes a list type reference for MarshalList.
type ListOptions struct {
	// Nullable lists marshal nil as null.
	Nullable bool
	// ElemNonNull lists are null when one of their elements is.
	ElemNonNull bool
	// Leaf lists hold scalars, which are marshaled in order without goroutines.
	Leaf bool
	// WorkerLimit and OmitPanicHandler are passed to graphql.MarshalSliceConcurrently.
	WorkerLimit      int64
	OmitPanicHandler bool
}

// MarshalList marshals a slice whose elements are marshaled by elem.
func MarshalList[EC, E any](elem Marshal[EC, E], o ListOptions) Marshal[EC, []E] {
	return func(ctx context.Context, ec EC, sel ast.SelectionSet, v []E) graphql.Marshaler {
		if o.Nullable && v == nil {
			return graphql.Null
		}
		var ret graphql.Array
		if o.Leaf {
			ret = make(graphql.Array, len(v))
			for i := range v {
				ret[i] = elem(ctx, ec, sel, v[i])
			}
		} else {
			ret = graphql.MarshalSliceConcurrently(ctx, len(v), o.WorkerLimit, o.OmitPanicHandler,
				func(ctx context.Context, i int) graphql.Marshaler {
					fc := graphql.GetFieldContext(ctx)
					fc.Result = &v[i]
					return elem(ctx, ec, sel, v[i])
				})
		}
		if o.ElemNonNull {
			for _, e := range ret {
				if e == graphql.Null {
					return graphql.Null
				}
			}
		}
		return ret
	}
}

// UnmarshalGQL unmarshals a value whose pointer implements graphql.Unmarshaler.
func UnmarshalGQL[EC, V any, PV interface {
	*V
	graphql.Unmarshaler
}]() Unmarshal[EC, V] {
	return func(ctx context.Context, _ EC, v any) (V, error) {
		var res V
		err := PV(&res).UnmarshalGQL(v)
		return res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalGQLPtr unmarshals a pointer to a value that implements graphql.Unmarshaler.
// Nullable references unmarshal null as nil.
func UnmarshalGQLPtr[EC, E any, PE interface {
	*E
	graphql.Unmarshaler
}](nullable bool) Unmarshal[EC, *E] {
	return func(ctx context.Context, _ EC, v any) (*E, error) {
		if nullable && v == nil {
			return nil, nil
		}
		res := new(E)
		err := PE(res).UnmarshalGQL(v)
		return res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalFunc unmarshals a value with a scalar unmarshaler such as
// graphql.UnmarshalString.
func UnmarshalFunc[EC, V any](f func(any) (V, error)) Unmarshal[EC, V] {
	return func(ctx context.Context, _ EC, v any) (V, error) {
		res, err := f(v)
		return res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalFuncPtr unmarshals a pointer with a scalar unmarshaler of its element.
// Nullable references unmarshal null as nil.
func UnmarshalFuncPtr[EC, E any](nullable bool, f func(any) (E, error)) Unmarshal[EC, *E] {
	return func(ctx context.Context, _ EC, v any) (*E, error) {
		if nullable && v == nil {
			return nil, nil
		}
		res, err := f(v)
		return &res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalInput unmarshals an input object whose Go type is T.
func UnmarshalInput[EC, T any](in *Input[EC]) Unmarshal[EC, T] {
	return func(ctx context.Context, ec EC, v any) (T, error) {
		res, err := InputValue[EC, T](ctx, ec, in, v)
		return res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalInputPtr unmarshals a pointer to an input object whose Go type is T.
// Nullable references unmarshal null as nil.
func UnmarshalInputPtr[EC, T any](nullable bool, in *Input[EC]) Unmarshal[EC, *T] {
	return func(ctx context.Context, ec EC, v any) (*T, error) {
		if nullable && v == nil {
			return nil, nil
		}
		res, err := InputValue[EC, T](ctx, ec, in, v)
		return &res, graphql.ErrorOnPath(ctx, err)
	}
}

// MarshalObject marshals a value of an object type, whose Object takes a pointer to it.
func MarshalObject[EC Context, E any](o *Object[EC]) Marshal[EC, E] {
	return func(ctx context.Context, ec EC, sel ast.SelectionSet, v E) graphql.Marshaler {
		return o.Marshal(ctx, ec, sel, &v)
	}
}

// MarshalObjectPtr marshals a pointer to a value of an object type, and nil as null.
func MarshalObjectPtr[EC Context, E any](nonNull bool, o *Object[EC]) Marshal[EC, *E] {
	return func(ctx context.Context, ec EC, sel ast.SelectionSet, v *E) graphql.Marshaler {
		if v == nil {
			return null(ctx, nonNull)
		}
		return o.Marshal(ctx, ec, sel, v)
	}
}

// UnmarshalList unmarshals a list whose elements are unmarshaled by elem. A single
// value is coerced to a list of one. Nullable references unmarshal null as nil.
func UnmarshalList[EC, E any](nullable bool, elem Unmarshal[EC, E]) Unmarshal[EC, []E] {
	return func(ctx context.Context, ec EC, v any) ([]E, error) {
		if nullable && v == nil {
			return nil, nil
		}
		vSlice := graphql.CoerceList(v)
		res := make([]E, len(vSlice))
		for i := range vSlice {
			ctx := graphql.WithPathContext(ctx, graphql.NewPathWithIndex(i))
			var err error
			if res[i], err = elem(ctx, ec, vSlice[i]); err != nil {
				return nil, err
			}
		}
		return res, nil
	}
}

// MarshalFuncContext marshals a value with a scalar marshaler that needs the context,
// such as graphql.MarshalFloatContext.
func MarshalFuncContext[EC, V any](
	nonNull bool,
	f func(V) graphql.ContextMarshaler,
) Marshal[EC, V] {
	return func(ctx context.Context, _ EC, _ ast.SelectionSet, v V) graphql.Marshaler {
		res := f(v)
		if res == graphql.Null {
			null(ctx, nonNull)
		}
		return graphql.WrapContextMarshaler(ctx, res)
	}
}

// MarshalFuncContextPtr is MarshalFuncContext for a pointer, and marshals nil as null.
func MarshalFuncContextPtr[EC, E any](
	nonNull bool,
	f func(E) graphql.ContextMarshaler,
) Marshal[EC, *E] {
	return func(ctx context.Context, _ EC, _ ast.SelectionSet, v *E) graphql.Marshaler {
		if v == nil {
			return null(ctx, nonNull)
		}
		res := f(*v)
		if res == graphql.Null {
			null(ctx, nonNull)
		}
		return graphql.WrapContextMarshaler(ctx, res)
	}
}

// UnmarshalFuncContext unmarshals a value with a scalar unmarshaler that needs the
// context, such as graphql.UnmarshalFloatContext.
func UnmarshalFuncContext[EC, V any](f func(context.Context, any) (V, error)) Unmarshal[EC, V] {
	return func(ctx context.Context, _ EC, v any) (V, error) {
		res, err := f(ctx, v)
		return res, graphql.ErrorOnPath(ctx, err)
	}
}

// UnmarshalFuncContextPtr is UnmarshalFuncContext for a pointer. Nullable references
// unmarshal null as nil.
func UnmarshalFuncContextPtr[EC, E any](
	nullable bool,
	f func(context.Context, any) (E, error),
) Unmarshal[EC, *E] {
	return func(ctx context.Context, _ EC, v any) (*E, error) {
		if nullable && v == nil {
			return nil, nil
		}
		res, err := f(ctx, v)
		return &res, graphql.ErrorOnPath(ctx, err)
	}
}
