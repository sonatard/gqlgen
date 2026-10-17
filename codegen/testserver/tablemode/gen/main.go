// Command gen generates the executors of the tablemode test server, as testdata/gqlgen.go
// does, with a plugin that changes the schema after gqlgen loads it, as a SchemaMutator
// may. The code is generated for the changed schema, while the executors validate
// requests against the schema of the sources, which the changes are not in.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"time"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file")
	flag.Parse()

	log.SetOutput(io.Discard)
	start := time.Now()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}
	if err := api.Generate(cfg, api.AddPlugin(guard{})); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
	fmt.Printf("Generated %s in %4.2fs\n", cfg.Exec.ImportPath(), time.Since(start).Seconds())
}

// guard changes the fields of the query type that a schema of the tablemode test server
// declares for it: it puts @auth on pluginGuarded and @trimArg on the argument of
// pluginTrimmed, makes pluginRequired non-null, removes pluginRemoved, and puts directives
// where their definitions do not allow them: @trimArg, which the schema declares on
// arguments only, on pluginMisplaced, @onObject and @onInput on pluginObjectOnField and
// pluginInputObjectOnField, @onField on the scalar Tagged and @onObject on MarkInput.v.
type guard struct{}

var _ plugin.SchemaMutator = guard{}

func (guard) Name() string { return "guard" }

func (guard) MutateSchema(s *ast.Schema) error {
	fields := s.Query.Fields
	if f := fields.ForName("pluginGuarded"); f != nil {
		f.Directives = append(f.Directives, directive(s, "auth", ast.LocationFieldDefinition))
	}
	if f := fields.ForName("pluginTrimmed"); f != nil {
		text := f.Arguments.ForName("text")
		text.Directives = append(text.Directives,
			directive(s, "trimArg", ast.LocationArgumentDefinition))
	}
	if f := fields.ForName("pluginRequired"); f != nil {
		f.Type = ast.NonNullNamedType(f.Type.Name(), nil)
	}
	if f := fields.ForName("pluginMisplaced"); f != nil {
		f.Directives = append(f.Directives, directive(s, "trimArg", ast.LocationFieldDefinition))
	}
	if f := fields.ForName("pluginObjectOnField"); f != nil {
		f.Directives = append(f.Directives, directive(s, "onObject", ast.LocationFieldDefinition))
	}
	if f := fields.ForName("pluginInputObjectOnField"); f != nil {
		f.Directives = append(f.Directives, directive(s, "onInput", ast.LocationFieldDefinition))
	}
	if typ := s.Types["Tagged"]; typ != nil {
		typ.Directives = append(typ.Directives, directive(s, "onField", ast.LocationScalar))
	}
	if typ := s.Types["MarkInput"]; typ != nil {
		v := typ.Fields.ForName("v")
		v.Directives = append(
			v.Directives,
			directive(s, "onObject", ast.LocationInputFieldDefinition),
		)
	}
	s.Query.Fields = slices.DeleteFunc(fields, func(f *ast.FieldDefinition) bool {
		return f.Name == "pluginRemoved"
	})
	return nil
}

// directive is the application of the directive called name, without arguments.
func directive(s *ast.Schema, name string, location ast.DirectiveLocation) *ast.Directive {
	return &ast.Directive{Name: name, Definition: s.Directives[name], Location: location}
}
