package exec

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
)

// arg is one argument of a field or directive, as Schema.Link reads it from the schema.
type arg[EC Context] struct {
	name string
	// directives are the directives applied to the argument. They receive the map of
	// raw arguments.
	directives []directive[EC]
	// nilOK is set when a directive may return nil for the argument, which is when
	// the argument's Go type can be nil.
	nilOK bool
	// withNull runs the directives even when the argument is absent, as set by
	// call_argument_directives_with_null.
	withNull bool
	// typ unmarshals the argument.
	typ *In[EC]
	// goType is the Go type of the argument that errors name, when it is not the name of
	// typ. See WithGoType.
	goType string
}

// WithGoType is an entry of Field.Args that also gives the Go type of the argument, which
// errors name, as the generated package writes it. Generated code gives it where the
// runtime would write the type otherwise: reflect does not tell []byte from []uint8, nor
// interface{} from any.
type WithGoType struct {
	// Arg is the entry of the argument, as Field.Args describes it.
	Arg    any
	GoType string
}

// parseArgs unmarshals rawArgs into a map that holds a value of the argument's Go type
// for every argument, absent ones included. withPath adds the name of each argument to
// the path of its errors. The functions mode leaves it out for the arguments of the
// directives in the schema, which it unmarshals in place.
func parseArgs[EC Context](
	ctx context.Context,
	ec EC,
	args []arg[EC],
	rawArgs map[string]any,
	withPath bool,
) (map[string]any, error) {
	res := make(map[string]any, len(args))
	for i := range args {
		v, err := args[i].parse(ctx, ec, rawArgs, withPath)
		if err != nil {
			return nil, err
		}
		res[args[i].name] = v
	}
	return res, nil
}

func (a *arg[EC]) parse(
	ctx context.Context,
	ec EC,
	rawArgs map[string]any,
	withPath bool,
) (any, error) {
	raw, ok := rawArgs[a.name]
	if len(a.directives) == 0 {
		if !ok {
			return a.typ.zero, nil
		}
		if withPath {
			ctx = graphql.WithPathContext(ctx, graphql.NewPathWithField(a.name))
		}
		return a.typ.unmarshal(ctx, ec, raw)
	}

	if !ok && !a.withNull {
		return a.typ.zero, nil
	}
	ctx = graphql.WithPathContext(ctx, graphql.NewPathWithField(a.name))
	next := func(ctx context.Context) (any, error) {
		// The directives receive rawArgs and may change it, so the argument is read when
		// they call next, as in the functions mode.
		raw, ok := rawArgs[a.name]
		if !ok {
			return a.typ.zero, nil
		}
		return a.typ.unmarshal(ctx, ec, raw)
	}
	tmp, err := chain(ec, rawArgs, a.directives, next, failure{zero: a.typ.zero})(ctx)
	if err != nil {
		return a.typ.zero, graphql.ErrorOnPath(ctx, err)
	}
	if a.typ.accept(tmp) {
		return tmp, nil
	}
	if a.nilOK && tmp == nil {
		return a.typ.zero, nil
	}
	goType := a.typ.name
	if a.goType != "" {
		goType = a.goType
	}
	return a.typ.zero, graphql.ErrorOnPath(ctx, fmt.Errorf(
		"unexpected type %T from directive, should be %s", tmp, goType))
}
