package codegen

import (
	"errors"
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
)

// checkTableMode returns an error when the schema uses a feature that exec.mode table
// does not support yet.
func checkTableMode(data *Data) error {
	if data.SubscriptionRoot != nil {
		return errors.New(
			"exec.mode table does not support subscriptions yet; use exec.mode functions",
		)
	}
	if data.Config.Federation.IsDefined() {
		return errors.New(
			"exec.mode table does not support federation yet; use exec.mode functions",
		)
	}
	for _, o := range data.Objects {
		for _, f := range o.Fields {
			if f.IsBatch() {
				return fmt.Errorf(
					"exec.mode table does not support batch resolvers yet (%s.%s); use exec.mode functions",
					o.Name,
					f.Name,
				)
			}
		}
	}
	return nil
}

// ObjectMarshal returns the Go expression that marshals value, selected by sel, as the
// object, interface or union type called name. value is empty for the root operation
// types. In table mode objects are marshaled by their table; interfaces and unions
// keep their generated function.
func (d *Data) ObjectMarshal(name, sel, value string) string {
	if def := d.Config.Schema.Types[name]; d.Config.Exec.IsTable() && def != nil &&
		def.Kind == ast.Object {
		if value == "" {
			value = "nil"
		}
		return fmt.Sprintf("object%s.Marshal(ctx, ec, %s, %s)", name, sel, value)
	}
	args := d.ECArg() + sel
	if value != "" {
		args += ", " + value
	}
	return fmt.Sprintf("%s_%s(ctx, %s)", d.ECDot(), name, args)
}

// TableMarshal returns the Go expression of the runtime combinator that marshals the
// type reference in table mode, or "" when the reference has a shape the combinators do
// not cover and keeps its generated function.
func (d *Data) TableMarshal(t *config.TypeReference) string {
	if !d.Config.Exec.IsTable() || t.CastType != nil || t.HasEnumValues() || t.IsRoot {
		return ""
	}
	if t.IsContext && t.Marshaler == nil {
		return ""
	}
	if t.IsPtrToSlice() || t.IsPtrToIntf() || t.IsPtrToPtr() {
		return ""
	}
	nonNull := t.GQL.NonNull
	lookup := templates.CurrentImports.LookupType
	ptr, isPtr := t.GO.(*types.Pointer)

	switch {
	case t.IsSlice():
		var opts []string
		if !nonNull {
			opts = append(opts, "Nullable: true")
		}
		if t.Elem().GQL.NonNull {
			opts = append(opts, "ElemNonNull: true")
		}
		if t.IsScalar() {
			opts = append(opts, "Leaf: true")
		}
		if d.Config.Exec.WorkerLimit != 0 {
			opts = append(opts, fmt.Sprintf("WorkerLimit: %d", d.Config.Exec.WorkerLimit))
		}
		if d.Config.OmitPanicHandler {
			opts = append(opts, "OmitPanicHandler: true")
		}
		return fmt.Sprintf(
			"exec.MarshalList(%s, exec.ListOptions{%s})",
			t.Elem().MarshalFunc(),
			strings.Join(opts, ", "),
		)
	case t.IsMarshaler:
		if isPtr {
			return fmt.Sprintf(
				"exec.MarshalSelfPtr[*executionContext, %s](%t)",
				lookup(ptr.Elem()),
				nonNull,
			)
		}
		if !t.IsNilable() {
			return fmt.Sprintf("exec.MarshalSelf[*executionContext, %s]()", lookup(t.GO))
		}
	case t.Marshaler != nil:
		if t.IsTargetNilable() {
			return ""
		}
		fn := "exec.MarshalFunc"
		if t.IsContext {
			fn = "exec.MarshalFuncContext"
		}
		if isPtr {
			return fmt.Sprintf(
				"%sPtr[*executionContext](%t, %s)",
				fn,
				nonNull,
				templates.Call(t.Marshaler),
			)
		}
		if !t.IsNilable() {
			return fmt.Sprintf(
				"%s[*executionContext](%t, %s)",
				fn,
				nonNull,
				templates.Call(t.Marshaler),
			)
		}
	case t.Definition.Kind == ast.Object:
		if t.Target == nil || config.IsNilable(t.Target) {
			return ""
		}
		if types.Identical(t.GO, t.Target) {
			return fmt.Sprintf(
				"exec.MarshalObject[*executionContext, %s](&object%s)",
				lookup(t.GO),
				t.Definition.Name,
			)
		}
		if isPtr && types.Identical(ptr.Elem(), t.Target) {
			return fmt.Sprintf(
				"exec.MarshalObjectPtr[*executionContext, %s](%t, &object%s)",
				lookup(ptr.Elem()), nonNull, t.Definition.Name,
			)
		}
	case t.Definition.Kind == ast.Interface || t.Definition.Kind == ast.Union:
		if types.IsInterface(t.GO) {
			return fmt.Sprintf("exec.MarshalInterface(%t, _%s)", nonNull, t.Definition.Name)
		}
	}
	return ""
}

// TableUnmarshal returns the Go expression of the runtime combinator that unmarshals the
// type reference in table mode, or "" when the reference keeps its generated function.
func (d *Data) TableUnmarshal(t *config.TypeReference) string {
	if !d.Config.Exec.IsTable() || t.CastType != nil || t.HasEnumValues() {
		return ""
	}
	if t.IsContext && t.Unmarshaler == nil {
		return ""
	}
	if t.IsPtrToSlice() || t.IsPtrToIntf() || t.IsPtrToPtr() {
		return ""
	}
	// The functions mode returns nil for null before anything else in these references.
	nullable := t.IsNilable() && !t.GQL.NonNull
	lookup := templates.CurrentImports.LookupType
	ptr, isPtr := t.GO.(*types.Pointer)

	switch {
	case t.IsSlice():
		return fmt.Sprintf("exec.UnmarshalList(%t, %s)", nullable, t.Elem().UnmarshalFunc())
	case t.Unmarshaler != nil:
		if t.IsTargetNilable() {
			return ""
		}
		fn := "exec.UnmarshalFunc"
		if t.IsContext {
			fn = "exec.UnmarshalFuncContext"
		}
		if isPtr {
			return fmt.Sprintf(
				"%sPtr[*executionContext](%t, %s)",
				fn,
				nullable,
				templates.Call(t.Unmarshaler),
			)
		}
		if !t.IsNilable() {
			return fmt.Sprintf("%s[*executionContext](%s)", fn, templates.Call(t.Unmarshaler))
		}
	case t.IsMarshaler:
		if isPtr {
			return fmt.Sprintf(
				"exec.UnmarshalGQLPtr[*executionContext, %s](%t)",
				lookup(ptr.Elem()),
				nullable,
			)
		}
		if !t.IsNilable() {
			return fmt.Sprintf("exec.UnmarshalGQL[*executionContext, %s]()", lookup(t.GO))
		}
	case t.Definition.Kind == ast.InputObject:
		if t.PointersInUnmarshalInput || t.IsMap() {
			return ""
		}
		if isPtr {
			return fmt.Sprintf(
				"exec.UnmarshalInputPtr[*executionContext, %s](%t, &input%s)",
				lookup(ptr.Elem()), nullable, t.Definition.Name,
			)
		}
		if !t.IsNilable() {
			return fmt.Sprintf(
				"exec.UnmarshalInput[*executionContext, %s](&input%s)",
				lookup(t.GO),
				t.Definition.Name,
			)
		}
	}
	return ""
}

// TableDirectiveDefs returns the directives that table mode declares an
// exec.DirectiveDef for: those of this schema that can be applied to fields, arguments
// and input objects, which are the places the tables run directives.
func (d *Data) TableDirectiveDefs() DirectiveList {
	return locationDirectives(
		d.Directives(),
		ast.LocationFieldDefinition,
		ast.LocationObject,
		ast.LocationArgumentDefinition,
		ast.LocationInputFieldDefinition,
		ast.LocationInputObject,
	)
}

func tableOutVar(t *config.TypeReference) string {
	return "out" + templates.UcFirst(t.MarshalFunc())
}

func tableInVar(t *config.TypeReference) string {
	return "in" + templates.UcFirst(t.UnmarshalFunc())
}

// TableOutTypes returns the types that fields return, once each, for which table mode
// declares an exec.Out.
func (d *Data) TableOutTypes() []*config.TypeReference {
	seen := map[string]bool{}
	var res []*config.TypeReference
	for _, o := range d.Objects {
		for _, f := range o.Fields {
			name := f.TypeReference.MarshalFunc()
			if !seen[name] {
				seen[name] = true
				res = append(res, f.TypeReference)
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].MarshalFunc() < res[j].MarshalFunc() })
	return res
}

// TableInTypes returns the types that arguments and input fields take, once each, for
// which table mode declares an exec.In.
func (d *Data) TableInTypes() []*config.TypeReference {
	seen := map[string]bool{}
	var res []*config.TypeReference
	add := func(t *config.TypeReference) {
		if name := t.UnmarshalFunc(); !seen[name] {
			seen[name] = true
			res = append(res, t)
		}
	}
	for _, o := range d.Objects {
		for _, f := range o.Fields {
			for _, a := range f.Args {
				add(a.TypeReference)
			}
		}
	}
	for _, in := range d.Inputs {
		if in.HasUnmarshal() {
			continue
		}
		for _, f := range in.Fields {
			add(f.TypeReference)
		}
	}
	for _, dir := range d.TableDirectiveDefs() {
		for _, a := range dir.Args {
			add(a.TypeReference)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].UnmarshalFunc() < res[j].UnmarshalFunc() })
	return res
}
