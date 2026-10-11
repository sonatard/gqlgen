package codegen

import (
	"embed"
	"io/fs"
	"slices"

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
)

// The templates of table mode, which differ from those of the functions mode in the files
// of the objects, the inputs and the fields, and in table_.gotpl.

//go:embed api!.gotpl internal!.gotpl directives_.gotpl table_.gotpl
var tableRootTemplateFS embed.FS

//go:embed args.gotpl field_.gotpl field_table.gotpl input_.gotpl input_table.gotpl interface.gotpl object_table.gotpl type.gotpl generated!.gotpl directives_.gotpl table_.gotpl
var tableSchemaTemplateFS embed.FS

//go:embed api!.gotpl internal!.gotpl args.gotpl field_.gotpl field_table.gotpl input_.gotpl input_table.gotpl interface.gotpl object_table.gotpl type.gotpl generated!.gotpl directives_.gotpl table_.gotpl
var tableSingleFileTemplateFS embed.FS

// tableData is what table mode keeps of the data.
type tableData struct {
	// TableLinkedSchema is the schema in SDL that the tables of table mode are linked
	// to, when it is not the schema of the sources. See tableLinkedSchema.
	TableLinkedSchema string
	// tableBuilt is the tableState of table mode when BuildData returned it.
	tableBuilt map[string]string
	// unmarshalerCalls holds the unmarshalers that the code that plugins generate calls.
	// See AddUnmarshalerCall.
	unmarshalerCalls map[string]bool
	// tableNamesKept are the names of the tables that table mode derived before it derived
	// them again. See tableRederive.
	tableNamesKept map[string]string
	// table is what table mode derives from the whole schema, which the files of the
	// follow-schema layout share. See tableInfo.
	table *tableInfo
	// tableGroups are the groups of tables that the generated file of this Data
	// builds: those of its own types for a file of the follow-schema layout, and all
	// of them for the root file and the single-file layout. See TableInitGroups.
	tableGroups [][]*Object
}

// tableBuild does in table mode what BuildData does for it, once d holds the data of the
// functions mode and its sources: it checks the directives, names the functions of the
// objects and inputs, writes the schema that the tables are linked to where it is not
// that of the sources, and keeps the state of the data that plugins must not change.
func (d *Data) tableBuild(sources []*ast.Source) error {
	cfg := d.Config
	if !cfg.Exec.IsTable() {
		return nil
	}
	if err := d.tableCheckDirectives(); err != nil {
		return err
	}
	tableFuncNames(cfg.Schema, d.Objects)
	tableFuncNames(cfg.Schema, d.Inputs)
	skipRuntime := func(name string) bool { return cfg.Directives[name].SkipRuntime }
	var err error
	if d.TableLinkedSchema, err = tableLinkedSchema(
		cfg.Schema,
		tableSources(sources, d.AugmentedSources, cfg.Exec.Dir()),
		skipRuntime,
	); err != nil {
		return err
	}
	d.tableBuilt = d.tableState()
	return nil
}

// tableGenerate does in table mode what GenerateCode does for it before it writes the
// files: it fails where plugins changed the data since BuildData in what the runtime reads
// from the schema, and has what table mode derives from the data derived again, as a
// plugin that asked for an unmarshaler had it derived from the data as it was then, which
// a later plugin may have changed.
func (d *Data) tableGenerate() error {
	if err := d.tableCheckState(); err != nil {
		return err
	}
	d.tableRederive()
	return nil
}

// tableShare gives build, the Data of a file of the follow-schema layout, which holds only
// some of the types, what table mode derives from the whole schema of d, and the groups of
// tables of its own types.
func (d *Data) tableShare(build *Data, filename string) {
	if !d.Config.Exec.IsTable() {
		return
	}
	info := d.tableInfo()
	build.table = info
	build.tableGroups = info.groupsByFile[filename]
}

// tableArgs is Args in table mode, which keeps the arguments of the fields in the tables of
// the objects, and unmarshals the arguments of the directives in the schema with their
// exec.DirectiveDef: it keeps the functions of the arguments of the directives of queries.
func (d *Data) tableArgs() map[string][]*FieldArgument {
	ret := map[string][]*FieldArgument{}
	for _, directive := range d.Directives() {
		if len(directive.Args) > 0 && directive.IsLocation(
			ast.LocationQuery,
			ast.LocationMutation,
			ast.LocationSubscription,
			ast.LocationField,
		) {
			ret[directive.ArgsFunc()] = directive.Args
		}
	}
	return ret
}

// renderExec renders a file of the executor of data with opts, which hold the templates of
// the functions mode. Table mode renders the templates table instead, without region
// tags. A file of table mode that holds an input with INPUT_OBJECT directives gets the
// import aliases of the functions mode, which names the Go type of the input with its
// alias in the message of the error of a directive that returns another type: the
// functions mode looks the packages up in another order, which numbers the aliases of
// packages with the same name otherwise.
func renderExec(data *Data, opts templates.Options, table fs.FS) error {
	if !data.Config.Exec.IsTable() {
		return templates.Render(opts)
	}
	functions := opts
	opts.TemplateFS = table
	opts.RegionTags = false
	_, err := fs.Stat(table, "input_table.gotpl")
	if err == nil && slices.ContainsFunc(data.Inputs, func(in *Object) bool {
		return !in.HasUnmarshal() && len(in.InputObjectDirectives()) > 0
	}) {
		functions.Data = data.functionsMode()
		if opts.Imports, err = templates.ImportsOf(functions); err != nil {
			return err
		}
	}
	return templates.Render(opts)
}

// functionsMode returns d with the configuration of the functions mode.
func (d *Data) functionsMode() *Data {
	cfg := *d.Config
	cfg.Exec.Mode = config.ExecModeFunctions
	functions := *d
	functions.Config = &cfg
	return &functions
}
