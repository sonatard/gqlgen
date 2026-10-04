package exec

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
)

// DirectiveDef describes a directive declared in the schema. Generated code declares one
// per directive.
type DirectiveDef[EC any] struct {
	Name string
	// Args are the arguments of the directive.
	Args []Arg[EC]
	// Call calls the directive's implementation with the unmarshaled arguments.
	Call func(ctx context.Context, ec EC, obj any, next graphql.Resolver, args map[string]any) (any, error)
}

// Directive is a directive applied in the schema, with the argument values written
// there.
type Directive[EC any] struct {
	Def *DirectiveDef[EC]
	// Args holds the argument values from the schema, or their defaults. Arguments
	// without either are absent.
	Args map[string]any
}

// With returns the application of d with the raw argument values args.
func (d *DirectiveDef[EC]) With(args map[string]any) Directive[EC] {
	return Directive[EC]{Def: d, Args: args}
}

// Dirs returns its arguments as a slice. Generated code uses it so that the slice's
// type does not have to be spelled out.
func Dirs[EC any](dirs ...Directive[EC]) []Directive[EC] {
	return dirs
}

// Chain wraps next with dirs. The first directive is called last, right before next,
// as the directives written first on a schema element are the innermost.
func Chain[EC any](ec EC, obj any, dirs []Directive[EC], next graphql.Resolver) graphql.Resolver {
	for i := range dirs {
		next = dirs[i].wrap(ec, obj, next)
	}
	return next
}

func (d *Directive[EC]) wrap(ec EC, obj any, next graphql.Resolver) graphql.Resolver {
	return func(ctx context.Context) (any, error) {
		var args map[string]any
		if len(d.Def.Args) > 0 {
			var err error
			if args, err = parseArgs(ctx, ec, d.Def.Args, d.Args, false); err != nil {
				return nil, err
			}
		}
		return d.Def.Call(ctx, ec, obj, next, args)
	}
}
