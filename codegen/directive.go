package codegen

import (
	"fmt"
	"go/types"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
)

type DirectiveList map[string]*Directive

// LocationDirectives filter directives by location
func (dl DirectiveList) LocationDirectives(location string) DirectiveList {
	return locationDirectives(dl, ast.DirectiveLocation(location))
}

type Directive struct {
	*ast.DirectiveDefinition
	Name string
	Args []*FieldArgument

	config.DirectiveConfig
}

// IsLocation check location directive
func (d *Directive) IsLocation(location ...ast.DirectiveLocation) bool {
	for _, l := range d.Locations {
		if slices.Contains(location, l) {
			return true
		}
	}

	return false
}

func locationDirectives(
	directives DirectiveList,
	location ...ast.DirectiveLocation,
) map[string]*Directive {
	mDirectives := make(map[string]*Directive)
	for name, d := range directives {
		if d.IsLocation(location...) {
			mDirectives[name] = d
		}
	}
	return mDirectives
}

// directiveArgVarNames returns the names of the variables that the generated code
// unmarshals the arguments args of a directive in the schema into, by argument. The
// variables are declared where the object that the directive receives is called obj,
// rawArgs, asMap or it, and where the values of later arguments are written with nil, true
// and false, so an argument with one of those names gets a suffix that no other argument
// has, rather than hide them.
func directiveArgVarNames(args ast.ArgumentDefinitionList) map[string]string {
	names := make(map[string]string, len(args))
	taken := make(map[string]bool, len(args))
	for _, arg := range args {
		taken[templates.ToGoPrivate(arg.Name)] = true
	}
	for _, arg := range args {
		v := templates.ToGoPrivate(arg.Name)
		switch v {
		case "obj", "rawArgs", "asMap", "it", "nil", "true", "false":
			for v += "Arg"; taken[v]; v += "Arg" {
			}
			taken[v] = true
		}
		names[arg.Name] = v
	}
	return names
}

func (b *builder) buildDirectives() (map[string]*Directive, error) {
	directives := make(map[string]*Directive, len(b.Schema.Directives))

	for name, dir := range b.Schema.Directives {
		if _, ok := directives[name]; ok {
			return nil, fmt.Errorf("directive with name %s already exists", name)
		}

		var args []*FieldArgument
		varNames := directiveArgVarNames(dir.Arguments)
		for _, arg := range dir.Arguments {
			tr, err := b.Binder.TypeReference(arg.Type, nil)
			if err != nil {
				return nil, err
			}

			newArg := &FieldArgument{
				ArgumentDefinition: arg,
				TypeReference:      tr,
				VarName:            varNames[arg.Name],
			}

			if arg.DefaultValue != nil {
				var err error
				newArg.Default, err = arg.DefaultValue.Value(nil)
				if err != nil {
					return nil, fmt.Errorf(
						"default value for directive argument %s(%s) is not valid: %w",
						dir.Name,
						arg.Name,
						err,
					)
				}
			}
			args = append(args, newArg)
		}

		directives[name] = &Directive{
			DirectiveDefinition: dir,
			Name:                name,
			Args:                args,
			DirectiveConfig:     b.Config.Directives[name],
		}
	}

	return directives, nil
}

func (b *builder) getDirectives(list ast.DirectiveList) ([]*Directive, error) {
	dirs := make([]*Directive, len(list))
	for i, d := range list {
		argValues := make(map[string]any, len(d.Arguments))
		for _, da := range d.Arguments {
			val, err := da.Value.Value(nil)
			if err != nil {
				return nil, err
			}
			argValues[da.Name] = val
		}
		def, ok := b.Directives[d.Name]
		if !ok {
			return nil, fmt.Errorf("directive %s not found", d.Name)
		}

		var args []*FieldArgument
		for _, a := range def.Args {
			value := a.Default
			if argValue, ok := argValues[a.Name]; ok {
				value = argValue
			}
			args = append(args, &FieldArgument{
				ArgumentDefinition: a.ArgumentDefinition,
				Value:              value,
				VarName:            a.VarName,
				TypeReference:      a.TypeReference,
			})
		}
		dirs[i] = &Directive{
			Name:                d.Name,
			Args:                args,
			DirectiveDefinition: list[i].Definition,
			DirectiveConfig:     b.Config.Directives[d.Name],
		}
	}

	return dirs, nil
}

func (d *Directive) ArgsFunc() string {
	if len(d.Args) == 0 {
		return ""
	}

	return "dir_" + d.Name + "_args"
}

func (d *Directive) CallArgs() string {
	args := []string{"ctx", "obj", "n"}

	for _, arg := range d.Args {
		args = append(
			args,
			fmt.Sprintf(
				"args[%q].(%s)",
				arg.Name,
				templates.CurrentImports.LookupType(arg.TypeReference.GO),
			),
		)
	}

	return strings.Join(args, ", ")
}

func (d *Directive) ResolveArgs(obj string, next int) string {
	args := []string{"ctx", obj, fmt.Sprintf("directive%d", next)}

	for _, arg := range d.Args {
		dArg := arg.VarName
		if arg.Value == nil && arg.Default == nil {
			dArg = "nil"
		}

		args = append(args, dArg)
	}

	return strings.Join(args, ", ")
}

// TableCallArgs is CallArgs for the Call function of an exec.DirectiveDef, which receives
// the object as directiveObj and the arguments as directiveArgs: names that a package of
// the Go types of the arguments is unlikely to have, which they would hide. An argument of
// the empty interface type is passed as it is, since it is nil when it is absent or null.
// An argument of another interface type is nil in the map when it is absent, as the
// functions mode passes nil then, so it is read without a type assertion that would
// panic on nil.
func (d *Directive) TableCallArgs() string {
	args := []string{"ctx", "directiveObj", "n"}
	for _, arg := range d.Args {
		goType := templates.CurrentImports.LookupType(arg.TypeReference.GO)
		switch iface, ok := types.Unalias(arg.TypeReference.GO).(*types.Interface); {
		case ok && iface.Empty():
			args = append(args, fmt.Sprintf("directiveArgs[%q]", arg.Name))
		case types.IsInterface(arg.TypeReference.GO):
			args = append(args, fmt.Sprintf(
				"func() (v %s) { v, _ = directiveArgs[%q].(%s); return v }()",
				goType, arg.Name, goType,
			))
		default:
			args = append(args, fmt.Sprintf("directiveArgs[%q].(%s)", arg.Name, goType))
		}
	}
	return strings.Join(args, ", ")
}

// TableVar returns the name of the field of the tables that holds the exec.DirectiveDef
// generated for the directive in table mode.
func (d *Directive) TableVar() string {
	return "directive" + d.CallName()
}

func (d *Directive) CallName() string {
	return ucFirst(d.Name)
}

func (d *Directive) Declaration() string {
	res := d.CallName() + " func(ctx context.Context, obj any, next graphql.Resolver"

	var resSb173 strings.Builder
	for _, arg := range d.Args {
		fmt.Fprintf(
			&resSb173,
			", %s %s",
			templates.ToGoPrivate(arg.Name),
			templates.CurrentImports.LookupType(arg.TypeReference.GO),
		)
	}
	res += resSb173.String()

	res += ") (res any, err error)"
	return res
}

func (d *Directive) IsBuiltIn() bool {
	return d.Implementation != nil
}

func (d *Directive) CallPath() string {
	if d.IsBuiltIn() {
		return "builtInDirective" + d.CallName()
	}

	return "ec.Directives." + d.CallName()
}

func (d *Directive) FunctionImpl() string {
	if d.Implementation == nil {
		return ""
	}

	return d.CallPath() + " = " + *d.Implementation
}
