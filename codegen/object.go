package codegen

import (
	"fmt"
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/99designs/gqlgen/codegen/config"
)

type GoFieldType int

const (
	GoFieldUndefined GoFieldType = iota
	GoFieldMethod
	GoFieldVariable
	GoFieldMap
)

type Object struct {
	*ast.Definition

	Type                     types.Type
	ResolverInterface        types.Type
	Root                     bool
	Fields                   []*Field
	Implements               []*ast.Definition
	DisableConcurrency       bool
	Stream                   bool
	Directives               []*Directive
	PointersInUnmarshalInput bool

	// tableFunc is the name of the function that table mode generates for the object or
	// input, set by tableFuncNames.
	tableFunc string
}

func (b *builder) buildObject(typ *ast.Definition) (*Object, error) {
	dirs, err := b.getDirectives(typ.Directives)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", typ.Name, err)
	}
	caser := cases.Title(language.English, cases.NoLower)
	obj := &Object{
		Definition:               typ,
		Root:                     b.Config.IsRoot(typ),
		DisableConcurrency:       typ == b.Schema.Mutation,
		Stream:                   typ == b.Schema.Subscription,
		Directives:               dirs,
		PointersInUnmarshalInput: b.Config.ReturnPointersInUnmarshalInput,
		ResolverInterface: types.NewNamed(
			types.NewTypeName(0, b.Config.Exec.Pkg(), caser.String(typ.Name)+"Resolver", nil),
			nil,
			nil,
		),
	}

	if !obj.Root {
		goObject, err := b.Binder.DefaultUserObject(typ.Name)
		if err != nil {
			return nil, err
		}
		obj.Type = goObject
	}

	for _, intf := range b.Schema.GetImplements(typ) {
		obj.Implements = append(obj.Implements, b.Schema.Types[intf.Name])
	}

	for _, field := range typ.Fields {
		if strings.HasPrefix(field.Name, "__") {
			continue
		}

		var f *Field
		f, err = b.buildField(obj, field)
		if err != nil {
			return nil, err
		}

		obj.Fields = append(obj.Fields, f)
	}

	return obj, nil
}

func (o *Object) Reference() types.Type {
	if config.IsNilable(o.Type) {
		return o.Type
	}
	return types.NewPointer(o.Type)
}

type Objects []*Object

func (o *Object) Implementors() string {
	satisfiedBy := strconv.Quote(o.Name)
	var satisfiedBySb100 strings.Builder
	for _, s := range o.Implements {
		satisfiedBySb100.WriteString(", " + strconv.Quote(s.Name))
	}
	satisfiedBy += satisfiedBySb100.String()
	return "[]string{" + satisfiedBy + "}"
}

func (o *Object) HasResolvers() bool {
	for _, f := range o.Fields {
		if f.IsResolver || f.IsBatch() {
			return true
		}
	}
	return false
}

func (o *Object) HasUnmarshal() bool {
	if o.IsMap() {
		return false
	}
	for method := range o.Type.(*types.Named).Methods() {
		if method.Name() == "UnmarshalGQL" {
			return true
		}
	}
	return false
}

func (o *Object) HasDirectives() bool {
	if len(o.Directives) > 0 {
		return true
	}
	for _, f := range o.Fields {
		if f.HasDirectives() {
			return true
		}
	}

	return false
}

// HasSubscriptionContextField reports whether this object is a streaming
// (subscription) root with at least one field annotated @subscriptionContext,
// or the global subscription_context_field option is enabled.
// Codegen uses this to decide whether to emit the event-context-aware
// dispatcher and the optional ExecWithEventContext method on the
// generated executableSchema. Returns false for non-streaming objects.
func (o *Object) HasSubscriptionContextField() bool {
	if !o.Stream {
		return false
	}
	for _, f := range o.Fields {
		if f.UsesSubscriptionContext() {
			return true
		}
	}
	return false
}

// InputObjectDirectives returns directives that should be executed at the INPUT_OBJECT level.
// This is used for input types to execute @directives placed on the input object itself,
// after all fields have been unmarshaled.
// See: https://github.com/99designs/gqlgen/issues/2281
func (o *Object) InputObjectDirectives() []*Directive {
	if o.Kind != ast.InputObject {
		return nil
	}
	var d []*Directive
	for _, dir := range o.Directives {
		if !dir.SkipRuntime && dir.IsLocation(ast.LocationInputObject) {
			d = append(d, dir)
		}
	}
	return d
}

func (o *Object) IsConcurrent() bool {
	for _, f := range o.Fields {
		if f.IsConcurrent() {
			return true
		}
	}
	return false
}

// InvalidsIncrement returns the Go statement that increments the invalids
// counter for this object's field set. Concurrent objects require atomic
// access; sequential objects use a plain increment.
func (o *Object) InvalidsIncrement(fieldSetVar string) string {
	if o.IsConcurrent() {
		return fmt.Sprintf("atomic.AddUint32(&%s.Invalids, 1)", fieldSetVar)
	}
	return fieldSetVar + ".Invalids++"
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
// which resolve to an empty value of the type. The Resolve of an object that has no other
// fields does not assert the object to its Go type.
func (o *Object) TableReadsObject() bool {
	if o.Root {
		return false
	}
	for _, f := range o.Fields {
		if !f.TypeReference.IsRoot {
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

func (o *Object) IsReserved() bool {
	return strings.HasPrefix(o.Name, "__")
}

func (o *Object) IsMap() bool {
	return o.Type == config.MapType
}

func (o *Object) Description() string {
	return o.Definition.Description
}

func (o *Object) HasField(name string) bool {
	for _, f := range o.Fields {
		if f.Name == name {
			return true
		}
	}

	return false
}

func (os Objects) ByName(name string) *Object {
	for i, o := range os {
		if strings.EqualFold(o.Name, name) {
			return os[i]
		}
	}
	return nil
}

func ucFirst(s string) string {
	if s == "" {
		return ""
	}

	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
