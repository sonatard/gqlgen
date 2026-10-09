package config

import (
	"errors"
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type ExecConfig struct {
	Package string     `yaml:"package,omitempty"`
	Layout  ExecLayout `yaml:"layout,omitempty"` // Default: single-file

	// Only for single-file layout:
	Filename string `yaml:"filename,omitempty"`

	// Only for follow-schema layout:
	FilenameTemplate string `yaml:"filename_template,omitempty"` // String template with {name} as placeholder for base name.
	DirName          string `yaml:"dir"`

	// Maximum number of goroutines in concurrency to use when running multiple child resolvers
	// Suppressing the number of goroutines generated can reduce memory consumption per request,
	// but processing time may increase due to the reduced number of concurrences
	// Default: 0 (unlimited)
	WorkerLimit uint `yaml:"worker_limit"`

	// EmbedSchemaDir is the directory, relative to the directory of the generated code,
	// that gqlgen copies the schema files outside that directory into, so that the
	// generated code embeds them with go:embed instead of including their text. Unset, the
	// text of such files is written into the generated code. gqlgen owns the directory: it
	// deletes the files in it that it did not write.
	EmbedSchemaDir string `yaml:"embed_schema_dir,omitempty"`
}

type ExecLayout string

var (
	// Write all generated code to a single file.
	ExecLayoutSingleFile ExecLayout = "single-file"
	// Write generated code to a directory, generating one Go source file for each GraphQL schema
	// file.
	ExecLayoutFollowSchema ExecLayout = "follow-schema"
)

func (r *ExecConfig) Check() error {
	if r.Layout == "" {
		r.Layout = ExecLayoutSingleFile
	}

	switch r.Layout {
	case ExecLayoutSingleFile:
		if r.Filename == "" {
			return errors.New("filename must be specified when using single-file layout")
		}
		if !strings.HasSuffix(r.Filename, ".go") {
			return errors.New(
				"filename should be path to a go source file when using single-file layout",
			)
		}
		r.Filename = abs(r.Filename)
	case ExecLayoutFollowSchema:
		if r.DirName == "" {
			return errors.New("dir must be specified when using follow-schema layout")
		}
		r.DirName = abs(r.DirName)
	default:
		return fmt.Errorf("invalid layout %s", r.Layout)
	}

	if strings.ContainsAny(r.Package, "./\\") {
		return errors.New(
			"package should be the output package name only, do not include the output filename",
		)
	}

	if r.Package == "" && r.Dir() != "" {
		r.Package = code.NameForDir(r.Dir())
	}

	if r.EmbedSchemaDir != "" {
		dir := filepath.ToSlash(filepath.Clean(r.EmbedSchemaDir))
		if filepath.IsAbs(r.EmbedSchemaDir) || strings.HasPrefix(dir, "/") ||
			dir == "." || dir == ".." || strings.HasPrefix(dir, "../") {
			return fmt.Errorf(
				"embed_schema_dir must be a directory inside the directory of the generated code, not %s",
				r.EmbedSchemaDir,
			)
		}
		r.EmbedSchemaDir = dir
	}

	return nil
}

// EmbedSchemaPath returns the absolute path of the directory that the schema files outside
// the directory of the generated code are copied into, or "" when none is configured. It
// works before Check, as LoadConfig leaves the copies out of the schema.
func (r *ExecConfig) EmbedSchemaPath() string {
	if r.EmbedSchemaDir == "" {
		return ""
	}
	var dir string
	switch {
	case r.Layout == ExecLayoutFollowSchema:
		dir = r.DirName
	case r.Filename != "":
		dir = filepath.Dir(r.Filename)
	default:
		return ""
	}
	return abs(filepath.Join(dir, r.EmbedSchemaDir))
}

func (r *ExecConfig) ImportPath() string {
	if r.Dir() == "" {
		return ""
	}
	return code.ImportPathForDir(r.Dir())
}

func (r *ExecConfig) Dir() string {
	switch r.Layout {
	case ExecLayoutSingleFile:
		if r.Filename == "" {
			return ""
		}
		return filepath.Dir(r.Filename)
	case ExecLayoutFollowSchema:
		return abs(r.DirName)
	default:
		panic("invalid layout " + r.Layout)
	}
}

func (r *ExecConfig) Pkg() *types.Package {
	if r.Dir() == "" {
		return nil
	}
	return types.NewPackage(r.ImportPath(), r.Package)
}

func (r *ExecConfig) IsDefined() bool {
	return r.Filename != "" || r.DirName != ""
}
