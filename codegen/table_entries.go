package codegen

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/99designs/gqlgen/codegen/templates"
)

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

// TableImplemented returns the implemented function of the exec.DirectiveDef generated for
// the directive in table mode: nil for a built-in directive, which always has an
// implementation.
func (d *Directive) TableImplemented() string {
	if d.IsBuiltIn() {
		return "nil"
	}
	return "func(ec *executionContext) bool { return " + d.CallPath() + " != nil }"
}

// TableBatch reports whether the field is resolved by the function of its batch resolver,
// which the functions mode generates for the fields of the objects that are not roots, in
// table mode. The table of the object calls it in place of the Resolve of the object, with
// the field as the request selects it. A field that returns a root operation type resolves
// to an empty value of the type instead, as in the functions mode.
func (f *Field) TableBatch() bool {
	return f.IsBatch() && !f.Object.Root && !f.TypeReference.IsRoot
}

// TableObj returns the expression of the field's object in table mode: obj, which the
// exec.Resolve of the object asserts to its Go type once, before it switches on the field.
func (f *Field) TableObj() string {
	return "obj"
}

// tableGetExpr returns the expression that reads the field from obj, the object with its
// Go type, in table mode, for fields bound to a struct field, or to a method that takes
// no context or arguments and returns no error. It returns "" for other fields, and for
// those whose Go type the marshaler of the field does not take, such as an Omittable that
// marshals itself: the functions mode returns the value as any, and the field checks its
// type when it resolves.
func (f *Field) tableGetExpr() string {
	if f.IsBatch() || f.HasHaser || f.VOkFunc || f.IsResolver || f.GoReceiverName != "obj" {
		return ""
	}
	if f.goType != nil && f.TypeReference != nil &&
		!types.AssignableTo(f.goType, f.TypeReference.GO) {
		return ""
	}
	switch {
	case f.IsVariable():
		return f.TableObj() + "." + f.GoFieldName
	case f.IsMethod() && f.NoErr && !f.MethodHasContext && len(f.Args) == 0:
		return f.TableObj() + "." + f.GoFieldName + "()"
	}
	return ""
}

// tableResolveBody returns the body of the case of the field in the exec.Resolve of its
// object in table mode, written as a single statement that reads the object from obj and
// the arguments from fieldArgs. It returns "" when the field needs more than one statement;
// the template then writes the body the functions mode uses, which reads the arguments from
// the field context: the runtime passes fieldArgs from the field context, so they are the
// same.
func (f *Field) tableResolveBody() string {
	if f.IsBatch() || f.HasHaser || f.VOkFunc {
		return ""
	}
	recv := f.GoReceiverName
	if recv == "obj" {
		recv = f.TableObj()
	}
	var body string
	switch {
	case f.IsResolver:
		args := []string{"ctx"}
		if !f.Object.Root {
			args = append(args, f.TableObj())
		}
		args = append(args, f.callArgExpressions("", "fieldArgs")...)
		caser := cases.Title(language.English, cases.NoLower)
		body = "return ec.Resolvers." + caser.String(
			f.Object.Name,
		) + "()." + f.GoFieldName + "(" + strings.Join(args, ", ") + ")"
	case f.IsMethod():
		call := recv + "." + f.GoFieldName + "(" + f.callArgs("fieldArgs") + ")"
		if f.NoErr {
			body = "return " + call + ", nil"
		} else {
			body = "return " + call
		}
	case f.IsVariable():
		body = "return " + recv + "." + f.GoFieldName + ", nil"
	default:
		return ""
	}
	if strings.Contains(body, "\n") {
		return ""
	}
	return body
}

// TableCaseBody returns the body of the case of the field in the exec.Resolve of its object
// in table mode. A field of a root operation type resolves to a fresh value of the type. A
// field read from its object declares the value with the Go type of the field's marshaler
// before it returns it as any, so that the compiler checks that the field has the type the
// marshaler marshals, as the typed call of the functions mode does. It returns "" when the
// field needs the body that the functions mode uses, which the template then writes.
func (f *Field) TableCaseBody() string {
	if f.TypeReference.IsRoot {
		if f.TypeReference.IsPtr() {
			return "return &" + tableType(f.TypeReference.Elem().GO) + "{}, nil"
		}
		return "return " + tableType(f.TypeReference.GO) + "{}, nil"
	}
	if get := f.tableGetExpr(); get != "" {
		return "var v " + tableType(f.TypeReference.GO) + " = " + get + "\nreturn v, nil"
	}
	return f.tableResolveBody()
}

// TableSetStmt returns the statement of the case of the input field in the Set of its input
// in table mode, which stores v into the input: m, its map, or p, the pointer to its Go
// type. It asserts v, which the runtime passes as any, to the Go type of the field's
// unmarshaler, so that the compiler checks that the field has the type the unmarshaler
// unmarshals, as the typed call of the functions mode does. A nil stores the zero value.
func (f *Field) TableSetStmt() string {
	goType := tableType(f.TypeReference.GO)
	switch {
	case f.Object.IsMap():
		return fmt.Sprintf("m[%q], _ = v.(%s)", f.Name, goType)
	case f.TypeReference.IsOmittable:
		return fmt.Sprintf("x, _ := v.(%s)\np.%s = graphql.OmittableOf(x)", goType, f.GoFieldName)
	default:
		return fmt.Sprintf("p.%s, _ = v.(%s)", f.GoFieldName, goType)
	}
}

// TableInputInvocation returns the resolver call that sets an input field from fieldValue
// in table mode, where the input being unmarshaled is the pointer inputValue, of type any.
// The functions mode calls it with data, which table mode does not name its variable, as
// it refers to the Go type of the input afterwards, which a package called data would hide.
func (f *Field) TableInputInvocation() string {
	caser := cases.Title(language.English, cases.NoLower)
	return fmt.Sprintf(
		"%s().%s(ctx, inputValue.(*%s), fieldValue)",
		caser.String(f.Object.Name),
		f.GoFieldName,
		templates.CurrentImports.LookupType(f.Object.Type),
	)
}

// TableVar returns the name of the field of the tables that holds the table generated
// for this object or input in table mode.
func (o *Object) TableVar() string {
	if o.Kind == ast.InputObject {
		return "input" + o.Name
	}
	return "object" + o.Name
}

// TableFunc returns the name of the function that table mode generates for this object
// or input: the exec.Resolve that resolves the fields of an object by their index, or
// the Set that stores the fields of an input.
func (o *Object) TableFunc() string {
	if o.tableFunc != "" {
		return o.tableFunc
	}
	return "_" + tableFuncName(o)
}

// tableFuncName is the name of the function of TableFunc without its leading underscore.
func tableFuncName(o *Object) string {
	if o.Kind == ast.InputObject {
		return o.Name + "_set"
	}
	return o.Name + "_resolve"
}

// tableFuncNames sets the names of the functions that table mode generates for objects:
// _User_resolve and _Pair_set, with underscores added while a type of schema has the name,
// whose function would take it, as the function of a union called User_resolve does.
func tableFuncNames(schema *ast.Schema, objects Objects) {
	for _, o := range objects {
		name := tableFuncName(o)
		for schema.Types[name] != nil {
			name += "_"
		}
		o.tableFunc = "_" + name
	}
}

// TableReadsObject reports whether a case of the exec.Resolve of this object in table mode
// reads the object: that of every field but those that return a root operation type,
// which resolve to an empty value of the type, and those of batch resolvers, which have no
// case. The Resolve of an object that has no other fields does not assert the object to
// its Go type.
func (o *Object) TableReadsObject() bool {
	if o.Root {
		return false
	}
	for _, f := range o.Fields {
		if !f.TypeReference.IsRoot && !f.TableBatch() {
			return true
		}
	}
	return false
}

// TableSetsFields reports whether the Set of this input in table mode stores any field:
// those that a resolver sets are stored by the resolver instead.
func (o *Object) TableSetsFields() bool {
	for _, f := range o.Fields {
		if !f.IsResolver {
			return true
		}
	}
	return false
}
