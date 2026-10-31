package smartsell

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// layoutLeaves are the leaf folders of this package: the sub-gate folders, each holding
// one sub-gate's settings and its pure readings of what sophos serves, and series, what
// the block serves every gate alike; this package composes them and wires them to the
// trade's state, rows and events. The layout rests on one direction of import: the
// composer imports every leaf and no leaf imports the composer, so no leaf can reach
// its unexported state or answer a tick of its own. A leaf may import a sibling leaf,
// never the composer.
var layoutLeaves = []string{
	"capitalprotection",
	"flatmarket",
	"freefallprevention",
	"earlytakeprofit",
	"lineardecline",
	"quietdecline",
	"series",
	"swingdecline",
}

// layoutFile is one parsed Go file of this package's directory tree.
type layoutFile struct {
	path string // slash path from this package's directory
	leaf string // the folder under this package; empty for the composer's own files
	tree *ast.File
}

// layoutFiles parses every Go file under this package's directory, test files and
// external test packages included and whatever build constraints they carry, so
// that no file can hide an import. testdata and the folders the go tool skips are
// left out. The files are read from disk: go test runs in the package directory.
func layoutFiles(t *testing.T) []layoutFile {
	t.Helper()
	fset := token.NewFileSet()
	var files []layoutFile
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != "." && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		tree, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(path)
		leaf, _, nested := strings.Cut(slash, "/")
		if !nested {
			leaf = ""
		}
		files = append(files, layoutFile{path: slash, leaf: leaf, tree: tree})
		return nil
	})
	if err != nil {
		t.Fatalf("reading the folder tree: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no Go file under the package directory: the test must run from it")
	}
	return files
}

// layoutFolders groups the files of the leaf folders by folder.
func layoutFolders(files []layoutFile) map[string][]layoutFile {
	folders := map[string][]layoutFile{}
	for _, f := range files {
		if f.leaf != "" {
			folders[f.leaf] = append(folders[f.leaf], f)
		}
	}
	return folders
}

// imports are the import paths of the file.
func (f layoutFile) imports() []string {
	var paths []string
	for _, spec := range f.tree.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

// declares is whether the file declares name at its top level, as a function, a
// type, a variable or a constant. A method of that name is not that declaration.
func (f layoutFile) declares(name string) bool {
	for _, decl := range f.tree.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name == name {
				return true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						return true
					}
				case *ast.ValueSpec:
					if slices.ContainsFunc(s.Names, func(id *ast.Ident) bool { return id.Name == name }) {
						return true
					}
				}
			}
		}
	}
	return false
}

// TestTheSubGateFoldersAreTheLayoutsLeaves keeps the guards below from passing for
// want of anything to read: every leaf of the layout still holds Go files, and each
// folder that does, leaves added later included, keeps its settings in constants.go
// and says what it holds in doc.go, as every package of the layout does.
func TestTheSubGateFoldersAreTheLayoutsLeaves(t *testing.T) {
	folders := layoutFolders(layoutFiles(t))
	for _, leaf := range layoutLeaves {
		if len(folders[leaf]) == 0 {
			t.Errorf("%s holds no Go file: the sub-gate left the layout", leaf)
		}
	}
	for _, leaf := range slices.Sorted(maps.Keys(folders)) {
		for _, name := range []string{"doc.go", "constants.go"} {
			if !slices.ContainsFunc(folders[leaf], func(f layoutFile) bool { return f.path == leaf+"/"+name }) {
				t.Errorf("%s has no %s", leaf, name)
			}
		}
	}
}

// TestNoLeafImportsTheComposer is the cycle rule the layout rests on. The compiler
// refuses a leaf that imports the composer through its own files, since the composer
// imports the leaf; it never sees an external test package of the leaf doing it, nor
// a file the build constraints leave out, and both are read here.
func TestNoLeafImportsTheComposer(t *testing.T) {
	composer := reflect.TypeOf(Result{}).PkgPath()
	folders := layoutFolders(layoutFiles(t))
	for _, leaf := range slices.Sorted(maps.Keys(folders)) {
		t.Run(leaf, func(t *testing.T) {
			for _, f := range folders[leaf] {
				if slices.Contains(f.imports(), composer) {
					t.Errorf("%s imports the composer %s: the composer wires its leaves, a leaf never reaches back", f.path, composer)
				}
			}
		})
	}
}

// TestOnlyTheComposerDeclaresApply keeps a tick decided in one place: the composer's
// own files declare Apply once, and no leaf, at any depth, declares a function,
// type, variable or constant of that name. A leaf answers a question of one
// sub-gate; the verdict, the rows and the sale are the composer's.
func TestOnlyTheComposerDeclaresApply(t *testing.T) {
	var composer []string
	for _, f := range layoutFiles(t) {
		switch {
		case !f.declares("Apply"):
		case f.leaf == "":
			composer = append(composer, f.path)
		default:
			t.Errorf("%s declares Apply: only the composer decides a tick", f.path)
		}
	}
	if len(composer) != 1 {
		t.Errorf("the composer's own files declare Apply in %v, want exactly one file", composer)
	}
}
