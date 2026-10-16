package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/plugin/stubgen"
)

func main() {
	stub := flag.String("stub", "", "name of stub file to generate")
	cfgPath := flag.String("config", "", "path to config file (use default if omitted)")
	bothModes := flag.Bool("both-modes", false,
		"also generate the executor with exec.mode table, into *.table.go files that build with the exectable tag")
	flag.Parse()

	log.SetOutput(io.Discard)

	start := graphql.Now()

	options := func() []api.Option {
		if *stub == "" {
			return nil
		}
		return []api.Option{api.AddPlugin(stubgen.New(*stub, "Stub"))}
	}

	// Table mode goes first: the follow-schema layout writes its root file as
	// root_.generated.go whatever the file names are, so it is renamed before the
	// functions mode writes its own.
	if *bothModes {
		if err := generateTableMode(*cfgPath, options()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	err = api.Generate(cfg, options()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}

	if *bothModes {
		if err := constrain("!exectable", functionsFiles(cfg)); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(3)
		}
	}

	fmt.Printf("Generated %s in %4.2fs\n", cfg.Exec.ImportPath(), time.Since(start).Seconds())
}

func loadConfig(cfgPath string) (*config.Config, error) {
	if cfgPath != "" {
		return config.LoadConfig(cfgPath)
	}
	return config.LoadConfigFromDefaultLocations()
}

// generateTableMode generates the executor of the config with exec.mode table, so that
// the same tests run against each mode: go test uses the functions mode, and go test
// -tags exectable uses table mode. The files of the executor are named *.table.go and
// get a build constraint for the tag. The resolver section is dropped, since the tests
// use the stub, which is the same for both modes.
func generateTableMode(cfgPath string, options []api.Option) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg.Exec.Mode = config.ExecModeTable
	followSchema := cfg.Exec.Layout == config.ExecLayoutFollowSchema
	var dir string
	if followSchema {
		cfg.Exec.FilenameTemplate = "{name}.table.go"
		dir = cfg.Exec.DirName
		if dir == "" {
			return errors.New("exec.dir is not set")
		}
	} else {
		cfg.Exec.Filename = strings.TrimSuffix(cfg.Exec.Filename, ".go") + ".table.go"
		dir = filepath.Dir(cfg.Exec.Filename)
	}
	// The federation plugin writes code that differs between the modes too.
	var federation string
	if cfg.Federation.IsDefined() {
		cfg.Federation.Filename = strings.TrimSuffix(cfg.Federation.Filename, ".go") + ".table.go"
		federation = cfg.Federation.Filename
	}
	cfg.Resolver = config.ResolverConfig{}
	// The files of the functions mode get their build constraint only after they are
	// generated, so validating the generated package would see both executors.
	cfg.SkipValidation = true
	// The packages are loaded as the tests of table mode build them.
	cfg.GoBuildTags = append(cfg.GoBuildTags, "exectable")

	// The files of table mode are written anew, so files left from a removed schema file
	// go. They are moved aside with the root file of the functions mode, which table
	// mode writes to first, and put back if table mode fails, so that a failure leaves
	// the files as they were rather than a package that builds with neither tag.
	root := filepath.Join(dir, "root_.generated.go")
	aside, _ := filepath.Glob(filepath.Join(dir, "*.table.go"))
	if federation != "" && filepath.Dir(federation) != dir {
		aside = append(aside, federation)
	}
	if followSchema {
		aside = append(aside, root)
	}
	b, err := moveAside(dir, aside)
	if err != nil {
		return err
	}
	files, err := generateTableFiles(cfg, options, dir, followSchema, federation)
	if err != nil {
		return errors.Join(err, b.restore())
	}
	if err := b.discard(); err != nil {
		return err
	}
	return constrain("exectable", files)
}

// generateTableFiles generates the files of table mode, and returns them.
func generateTableFiles(cfg *config.Config, options []api.Option, dir string, followSchema bool, federation string) ([]string, error) {
	if err := api.Generate(cfg, options...); err != nil {
		return nil, err
	}
	if followSchema {
		root := filepath.Join(dir, "root_.generated.go")
		if _, err := os.Stat(root); err == nil {
			if err := os.Rename(root, filepath.Join(dir, "root_.table.go")); err != nil {
				return nil, err
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.table.go"))
	if federation != "" && filepath.Dir(federation) != dir {
		files = append(files, federation)
	}
	if !anyContains(files, "exec.Object[") {
		return nil, fmt.Errorf("table mode wrote no executor to %s/*.table.go", dir)
	}
	return files, nil
}

// aside holds files moved out of their directory while table mode is generated.
type aside struct {
	dir   string
	files []string
}

// moveAside moves the existing files to a temporary directory in dir.
func moveAside(dir string, files []string) (*aside, error) {
	tmp, err := os.MkdirTemp(dir, ".both-modes-")
	if err != nil {
		return nil, err
	}
	b := &aside{dir: tmp}
	for i, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		if err := os.Rename(f, filepath.Join(tmp, strconv.Itoa(i))); err != nil {
			return nil, errors.Join(err, b.restore())
		}
		b.files = append(b.files, f)
	}
	return b, nil
}

// restore removes what a failed generation left in place of the files and puts them
// back. Other files of table mode that the failed generation wrote are removed too.
func (b *aside) restore() error {
	var errs []error
	written, _ := filepath.Glob(filepath.Join(filepath.Dir(b.dir), "*.table.go"))
	for _, f := range written {
		errs = append(errs, os.Remove(f))
	}
	for i, f := range b.files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
		errs = append(errs, os.Rename(filepath.Join(b.dir, strconv.Itoa(i)), f))
	}
	errs = append(errs, os.RemoveAll(b.dir))
	return errors.Join(errs...)
}

// discard removes the files moved aside.
func (b *aside) discard() error {
	return os.RemoveAll(b.dir)
}

// functionsFiles returns the files the functions mode wrote the executor to: exec.filename,
// or with the follow-schema layout the files of exec.filename_template, *.generated.go by
// default, and root_.generated.go whatever the template, in exec.dir.
func functionsFiles(cfg *config.Config) []string {
	var federation []string
	if cfg.Federation.IsDefined() {
		federation = []string{cfg.Federation.Filename}
	}
	if cfg.Exec.Layout != config.ExecLayoutFollowSchema {
		return append([]string{cfg.Exec.Filename}, federation...)
	}
	template := cfg.Exec.FilenameTemplate
	if template == "" {
		template = "{name}.generated.go"
	}
	files, _ := filepath.Glob(filepath.Join(cfg.Exec.DirName, strings.ReplaceAll(template, "{name}", "*")))
	root := filepath.Join(cfg.Exec.DirName, "root_.generated.go")
	if _, err := os.Stat(root); err == nil && !anyEquals(files, root) {
		files = append(files, root)
	}
	return append(files, federation...)
}

// constrain puts the build constraint tag on the files, unless they have one.
func constrain(tag string, files []string) error {
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if bytes.HasPrefix(b, []byte("//go:build ")) {
			continue
		}
		b = append([]byte("//go:build "+tag+"\n\n"), b...)
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func anyContains(files []string, s string) bool {
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil && bytes.Contains(b, []byte(s)) {
			return true
		}
	}
	return false
}

func anyEquals(files []string, s string) bool {
	for _, f := range files {
		if f == s {
			return true
		}
	}
	return false
}
