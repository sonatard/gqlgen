package codegen

import "embed"

// The templates of table mode, which differ from those of the functions mode in the files
// of the objects, the inputs and the fields, and in table_.gotpl.

//go:embed api!.gotpl internal!.gotpl directives_.gotpl table_.gotpl
var tableRootTemplateFS embed.FS

//go:embed args.gotpl field_.gotpl field_table.gotpl input_.gotpl input_table.gotpl interface.gotpl object_table.gotpl type.gotpl generated!.gotpl directives_.gotpl table_.gotpl
var tableSchemaTemplateFS embed.FS

//go:embed api!.gotpl internal!.gotpl args.gotpl field_.gotpl field_table.gotpl input_.gotpl input_table.gotpl interface.gotpl object_table.gotpl type.gotpl generated!.gotpl directives_.gotpl table_.gotpl
var tableSingleFileTemplateFS embed.FS
