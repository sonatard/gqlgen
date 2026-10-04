package exec

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql"
)

// Marshal converts V into a response value. Generated code passes its marshal
// functions.
type Marshal[EC, V any] func(ctx context.Context, ec EC, sel ast.SelectionSet, v V) graphql.Marshaler

// Context is implemented by the generated executionContext, through the
// graphql.ExecutionContextState it embeds.
type Context interface {
	OpCtx() *graphql.OperationContext
	AddDeferred(n int32)
	ProcessDeferredGroup(dg graphql.DeferredGroup)
}

// fieldMiddleware is implemented by the generated executionContext when the schema
// declares directives that queries may put on fields. FieldMiddleware applies them.
type fieldMiddleware interface {
	FieldMiddleware(ctx context.Context, obj any, next graphql.Resolver) graphql.Resolver
}

// Object describes how a GraphQL object type is resolved from its Go value.
//
// Generated code declares one Object per object type and calls Init with its fields in
// an init function, because fields refer to the objects of their child types. The
// object is passed around as any, so that one instantiation of the runtime serves every
// object type; the root operation types have no value and pass nil.
type Object[EC Context] struct {
	Name         string
	Implementors []string
	// Root is set for the query and mutation types. Their fields run inside the root
	// field middleware. The subscription type is resolved by Subscribe instead of
	// Marshal.
	Root bool
	// OmitPanicHandler is set when the generated code was configured not to recover
	// panics in resolvers.
	OmitPanicHandler bool

	fields []Field[EC]
	index  map[string]*Field[EC]
}

// Field describes one field of an object.
type Field[EC Context] struct {
	Name string
	// NonNull is set when the schema declares the field non-null.
	NonNull bool
	// Concurrent fields are resolved in their own goroutine, and may be deferred.
	Concurrent bool
	// RootValue is set for fields that return a root operation type. Get returns the
	// empty root value, which is marshaled without middleware or null checks.
	RootValue  bool
	IsMethod   bool
	IsResolver bool

	// Exactly one of Children, Leaf and Abstract describes the field's type. They decide
	// what FieldContext.Child does.
	//
	// Children is the object the field returns.
	Children ChildFields[EC]
	// Leaf is the name of a type without fields, such as a scalar, enum or union.
	Leaf string
	// Abstract is the kind of a type with fields that is not an object, such as INTERFACE.
	Abstract string

	// Args are the field's arguments.
	Args []Arg[EC]
	// Directives are the directives the schema declares on the field. The FIELD
	// directives of the query are applied around them by the executionContext's
	// FieldMiddleware.
	Directives []Directive[EC]
	// Get reads the field from its object, for struct fields and methods that need
	// neither a context nor arguments and cannot fail. Other fields set Resolve.
	Get func(obj any) any
	// Resolve resolves the field with the arguments unmarshaled into the field context.
	// It returns any, so that directives and middleware can replace the value.
	Resolve func(ctx context.Context, ec EC, obj any, args map[string]any) (any, error)
	// Out marshals the value of the field's Go type. A value of another type that
	// implements graphql.Marshaler is used as it is.
	Out *Out[EC]
	// Stream is set for the fields of the subscription type. It receives the events
	// from the channel the resolver returns, and Out marshals each of them.
	Stream *Stream
}

// ChildFields builds the field context of a field selected under an object. Object
// implements it.
type ChildFields[EC Context] interface {
	ChildFieldContext(
		ctx context.Context,
		ec EC,
		field graphql.CollectedField,
	) (*graphql.FieldContext, error)
}

// Out marshals the values of one Go type, given as any. Generated code declares one per
// type that fields return, with OutOf.
type Out[EC any] struct {
	accept  func(v any) bool
	marshal func(ctx context.Context, ec EC, sel ast.SelectionSet, v any) graphql.Marshaler
	name    string
}

// OutFor is the Out of the values of type V. V only adds to the static type, so that
// FieldGet can check at compile time that a getter returns the type the Out marshals.
type OutFor[EC, V any] struct{ *Out[EC] }

// OutOf returns the Out of the values that m marshals.
func OutOf[EC, V any](m Marshal[EC, V]) OutFor[EC, V] {
	var zero V
	return OutFor[EC, V]{&Out[EC]{
		accept: func(v any) bool {
			_, ok := v.(V)
			return ok
		},
		marshal: func(ctx context.Context, ec EC, sel ast.SelectionSet, v any) graphql.Marshaler {
			return m(ctx, ec, sel, v.(V))
		},
		name: fmt.Sprintf("%T", zero),
	}}
}

// FieldGet completes f to read the field with get from its object, of type T, and to
// marshal the value with out. The compiler checks that get returns the type out
// marshals; at run time the object and the value are passed as any.
func FieldGet[EC Context, T, V any](f Field[EC], out OutFor[EC, V], get func(T) V) Field[EC] {
	f.Out = out.Out
	f.Get = func(obj any) any { return get(obj.(T)) }
	return f
}

// Init sets the fields of the object. Generated code calls it once, from an init
// function.
func (o *Object[EC]) Init(fields []Field[EC]) {
	o.fields = fields
	o.index = make(map[string]*Field[EC], len(fields))
	for i := range fields {
		o.index[fields[i].Name] = &o.fields[i]
	}
}

// ParseArgs unmarshals the raw arguments of the field called name. The generated
// complexity function uses it.
func (o *Object[EC]) ParseArgs(
	ctx context.Context,
	ec EC,
	name string,
	rawArgs map[string]any,
) (map[string]any, error) {
	f := o.index[name]
	if f == nil {
		return nil, fmt.Errorf("no field named %q was found under type %s", name, o.Name)
	}
	return ParseArgs(ctx, ec, f.Args, rawArgs)
}

// ChildFieldContext returns the field context of field, selected under this object. It
// backs FieldContext.Child for fields that return this object.
func (o *Object[EC]) ChildFieldContext(
	ctx context.Context,
	ec EC,
	field graphql.CollectedField,
) (*graphql.FieldContext, error) {
	f := o.index[field.Name]
	if f == nil {
		return nil, fmt.Errorf("no field named %q was found under type %s", field.Name, o.Name)
	}
	return o.fieldContext(ctx, ec, f, field)
}

// Marshal resolves the fields of obj selected by sel.
func (o *Object[EC]) Marshal(
	ctx context.Context,
	ec EC,
	sel ast.SelectionSet,
	obj any,
) graphql.Marshaler {
	oc := ec.OpCtx()
	fields := graphql.CollectFields(oc, sel, o.Implementors)
	if o.Root {
		ctx = graphql.WithFieldContext(ctx, &graphql.FieldContext{Object: o.Name})
	}

	out := graphql.NewFieldSet(fields)
	var (
		deferred *graphql.FieldSet
		views    map[string]*graphql.FieldSetView
	)
	for i, field := range fields {
		if field.Name == "__typename" {
			out.Values[i] = graphql.MarshalString(o.Name)
			continue
		}
		f := o.index[field.Name]
		if f == nil {
			panic("unknown field " + strconv.Quote(field.Name))
		}

		var rootCtx context.Context
		if o.Root {
			rootCtx = graphql.WithRootFieldContext(ctx, &graphql.RootFieldContext{
				Object: field.Name,
				Field:  field,
			})
		}

		if !f.Concurrent {
			if o.Root {
				out.Values[i] = oc.RootResolverMiddleware(
					rootCtx,
					func(ctx context.Context) graphql.Marshaler {
						return o.resolveField(ctx, ec, f, field, obj)
					},
				)
			} else {
				out.Values[i] = o.resolveField(ctx, ec, f, field, obj)
			}
			if f.isInvalid(out.Values[i]) {
				atomic.AddUint32(&out.Invalids, 1)
			}
			continue
		}

		resolve := func(ctx context.Context, fs *graphql.FieldSet) (res graphql.Marshaler) {
			if !o.OmitPanicHandler {
				defer func() {
					if r := recover(); r != nil {
						oc.Error(ctx, oc.Recover(ctx, r))
					}
				}()
			}
			res = o.resolveField(ctx, ec, f, field, obj)
			if f.isInvalid(res) {
				atomic.AddUint32(&fs.Invalids, 1)
			}
			return res
		}

		if o.Root {
			out.Concurrently(i, func(context.Context) graphql.Marshaler {
				return oc.RootResolverMiddleware(
					rootCtx,
					func(ctx context.Context) graphql.Marshaler {
						return resolve(ctx, out)
					},
				)
			})
			continue
		}

		if field.IsDeferred() {
			if deferred == nil {
				deferred = graphql.NewFieldSet(nil)
				views = make(map[string]*graphql.FieldSetView)
			}
			deferred.AddField(field)
			fieldIndex := len(deferred.Values) - 1
			fs := deferred
			deferred.Concurrently(fieldIndex, func(ctx context.Context) graphql.Marshaler {
				return resolve(ctx, fs)
			})
			for _, deferrable := range field.Deferrables {
				view, ok := views[deferrable.Label]
				if !ok {
					view = deferred.NewView()
					views[deferrable.Label] = view
				}
				view.AddIndices(fieldIndex)
			}
			// The field is sent with its deferred group, not in this response.
			out.Values[i] = graphql.Null
			continue
		}

		out.Concurrently(i, func(ctx context.Context) graphql.Marshaler {
			return resolve(ctx, out)
		})
	}

	out.Dispatch(ctx)
	if out.Invalids > 0 {
		return graphql.Null
	}

	if len(views) > 0 {
		ec.AddDeferred(int32(min(len(views), math.MaxInt32)))
		ec.ProcessDeferredGroup(graphql.DeferredGroup{
			Defers:   views,
			Path:     graphql.GetPath(ctx),
			FieldSet: deferred,
			Context:  ctx,
		})
	}

	return out
}

// isInvalid reports whether a resolved value makes the object null: a null in a
// non-null field, or a null that a field marked non-null at runtime asks to propagate.
func (f *Field[EC]) isInvalid(res graphql.Marshaler) bool {
	if f.NonNull {
		return res == graphql.Null
	}
	return res == graphql.RequiredNull
}

// resolveField resolves one field of obj, as graphql.ResolveField does for the
// functions mode.
func (o *Object[EC]) resolveField(
	ctx context.Context,
	ec EC,
	f *Field[EC],
	field graphql.CollectedField,
	obj any,
) (ret graphql.Marshaler) {
	oc := ec.OpCtx()
	fc, err := o.fieldContext(ctx, ec, f, field)
	if err != nil {
		return graphql.Null
	}
	ctx = graphql.WithFieldContext(ctx, fc)

	if !o.OmitPanicHandler {
		defer func() {
			if r := recover(); r != nil {
				oc.Error(ctx, oc.Recover(ctx, r))
				ret = graphql.Null
			}
		}()
	}

	if f.RootValue {
		return f.marshal(ctx, ec, field.Selections, fc, f.Get(obj))
	}

	res, ctx, err := f.resolve(ctx, ec, obj)
	if err != nil {
		oc.Error(ctx, graphql.AddFieldLocationToError(ctx, err))
		if fc.NonNull && !f.NonNull {
			return graphql.RequiredNull
		}
		return graphql.Null
	}
	if res == nil {
		if f.NonNull || fc.NonNull {
			if !graphql.HasFieldError(ctx, fc) {
				graphql.AddErrorf(ctx, "must not be null")
			}
		}
		if fc.NonNull && !f.NonNull {
			return graphql.RequiredNull
		}
		return graphql.Null
	}
	return f.marshal(ctx, ec, field.Selections, fc, res)
}

// resolve runs the resolver of f on obj inside the field's directives, the FIELD
// directives of the query and the resolver middleware. It also returns the context that
// the middleware passed to the resolver, which the children of the field use.
func (f *Field[EC]) resolve(ctx context.Context, ec EC, obj any) (any, context.Context, error) {
	next := func(rctx context.Context) (any, error) {
		ctx = rctx // use context from middleware stack in children
		if f.Get != nil {
			return f.Get(obj), nil
		}
		var args map[string]any
		if len(f.Args) > 0 {
			// Middleware may have replaced the field context.
			args = graphql.GetFieldContext(rctx).Args
		}
		return f.Resolve(rctx, ec, obj, args)
	}
	next = Chain(ec, obj, f.Directives, next)
	if fm, ok := any(ec).(fieldMiddleware); ok {
		next = fm.FieldMiddleware(ctx, obj, next)
	}

	res, err := ec.OpCtx().ResolverMiddleware(ctx, next)
	return res, ctx, err
}

// marshal marshals the resolved value res of f.
func (f *Field[EC]) marshal(
	ctx context.Context,
	ec EC,
	sel ast.SelectionSet,
	fc *graphql.FieldContext,
	res any,
) graphql.Marshaler {
	if f.Out.accept(res) {
		fc.Result = res
		return f.Out.marshal(ctx, ec, sel, res)
	}
	if m, ok := res.(graphql.Marshaler); ok {
		fc.Result = m
		return m
	}
	graphql.AddErrorf(
		ctx,
		`unexpected type %T from middleware/directive chain, should be %s`,
		res,
		f.Out.name,
	)
	return graphql.Null
}

// fieldContext builds the field context of f and unmarshals its arguments.
func (o *Object[EC]) fieldContext(
	ctx context.Context,
	ec EC,
	f *Field[EC],
	field graphql.CollectedField,
) (fc *graphql.FieldContext, err error) {
	fc = &graphql.FieldContext{
		Object:     o.Name,
		Field:      field,
		IsMethod:   f.IsMethod,
		IsResolver: f.IsResolver,
		Child:      f.child(ec),
	}
	if len(f.Args) == 0 {
		return fc, nil
	}

	oc := ec.OpCtx()
	if !o.OmitPanicHandler {
		defer func() {
			if r := recover(); r != nil {
				err = oc.Recover(ctx, r)
				oc.Error(ctx, err)
			}
		}()
	}
	ctx = graphql.WithFieldContext(ctx, fc)
	if fc.Args, err = ParseArgs(ctx, ec, f.Args, field.ArgumentMap(oc.Variables)); err != nil {
		oc.Error(ctx, err)
		return fc, err
	}
	return fc, nil
}

// child returns the FieldContext.Child function of f.
func (f *Field[EC]) child(
	ec EC,
) func(context.Context, graphql.CollectedField) (*graphql.FieldContext, error) {
	switch {
	case f.Children != nil:
		children := f.Children
		return func(ctx context.Context, field graphql.CollectedField) (*graphql.FieldContext, error) {
			return children.ChildFieldContext(ctx, ec, field)
		}
	case f.Abstract != "":
		kind := f.Abstract
		return func(context.Context, graphql.CollectedField) (*graphql.FieldContext, error) {
			return nil, errors.New("FieldContext.Child cannot be called on type " + kind)
		}
	default:
		leaf := f.Leaf
		return func(context.Context, graphql.CollectedField) (*graphql.FieldContext, error) {
			return nil, errors.New("field of type " + leaf + " does not have child fields")
		}
	}
}
