package templates

import (
	"errors"
	"fmt"
	"go/types"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type Import struct {
	Name  string
	Path  string
	Alias string

	// given is set for the imports that Options.Imports gives, which yield their alias to
	// a package that the templates reserve it for.
	given bool
}

type Imports struct {
	imports  []*Import
	destDir  string
	packages *code.Packages
}

var AnyType = types.Universe.Lookup("any").Type()

func (i *Import) String() string {
	if strings.HasSuffix(i.Path, i.Alias) && i.Alias == i.Name {
		return strconv.Quote(i.Path)
	}

	return i.Alias + " " + strconv.Quote(i.Path)
}

func (s *Imports) String() string {
	res := ""
	var resSb35 strings.Builder
	for i, imp := range s.imports {
		if i != 0 {
			resSb35.WriteString("\n")
		}
		resSb35.WriteString(imp.String())
	}
	res += resSb35.String()
	return res
}

func (s *Imports) Reserve(path string, aliases ...string) (string, error) {
	if path == "" {
		panic("empty ambient import")
	}

	// if we are referencing our own package we don't need an import
	if code.ImportPathForDir(s.destDir) == path {
		return "", nil
	}

	name := s.packages.NameForPackage(path)
	var alias string
	if len(aliases) != 1 {
		alias = name
	} else {
		alias = aliases[0]
	}

	if existing := s.findByPath(path); existing != nil {
		if existing.Alias == alias {
			return "", nil
		}
		if !existing.given || s.findByAlias(alias) != nil {
			return "", errors.New("ambient import already exists")
		}
		existing.Alias = alias
		existing.given = false
		return "", nil
	}

	taken := s.findByAlias(alias)
	if taken != nil && !taken.given {
		return "", errors.New("ambient import collides on an alias")
	}

	s.imports = append(s.imports, &Import{
		Name:  name,
		Path:  path,
		Alias: alias,
	})
	if taken != nil {
		// The given import takes another alias, as a lookup after the reservation would
		// give it.
		taken.Alias = ""
		taken.Alias = s.freeAlias(taken.Name)
		taken.given = false
	}

	return "", nil
}

func (s *Imports) Lookup(path string) string {
	if path == "" {
		return ""
	}

	path = code.NormalizeVendor(path)

	// if we are referencing our own package we don't need an import
	if code.ImportPathForDir(s.destDir) == path {
		return ""
	}

	if existing := s.findByPath(path); existing != nil {
		return existing.Alias
	}

	imp := &Import{
		Name: s.packages.NameForPackage(path),
		Path: path,
	}
	s.imports = append(s.imports, imp)
	imp.Alias = s.freeAlias(imp.Name)

	return imp.Alias
}

// freeAlias returns name, or name with the lowest number after it that no import has as
// its alias.
func (s *Imports) freeAlias(name string) string {
	alias := name
	i := 1
	for s.findByAlias(alias) != nil {
		alias = name + strconv.Itoa(i)
		i++
		if i > 1000 {
			panic(fmt.Errorf("too many collisions, last attempt was %s", alias))
		}
	}
	return alias
}

func (s *Imports) LookupType(t types.Type) string {
	if iface, ok := types.Unalias(t).(*types.Interface); ok && iface.Empty() {
		t = AnyType
	}
	return types.TypeString(t, func(i *types.Package) string {
		return s.Lookup(i.Path())
	})
}

func (s Imports) findByPath(importPath string) *Import {
	for _, imp := range s.imports {
		if imp.Path == importPath {
			return imp
		}
	}
	return nil
}

func (s Imports) findByAlias(alias string) *Import {
	for _, imp := range s.imports {
		if imp.Alias == alias {
			return imp
		}
	}
	return nil
}
