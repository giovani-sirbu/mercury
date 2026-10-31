package cooldown

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

// layoutLeaves are the folders of the Cooldown flag's gates under this package.
// This package is their shared vocabulary: the gate names, the kinds, the row
// prefixes and marker, and the event documents with their constructors. Each leaf
// imports it and holds its gate's hold and the settings that hold reads, so no
// setting is global to the flag. The vocabulary imports no leaf, and the leaves
// import none of each other.
var layoutLeaves = []string{"depthpriority", "depthspacing", "firstfill"}

// layoutFile is one parsed Go file of this package's directory tree.
type layoutFile struct {
	path string // slash path from this package's directory
	leaf string // the folder under this package; empty for the vocabulary's own files
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

// leafImports are the leaves, by folder name, the file imports.
func (f layoutFile) leafImports() []string {
	prefix := reflect.TypeOf(FirstFillEvent{}).PkgPath() + "/"
	var leaves []string
	for _, path := range f.imports() {
		if rest, under := strings.CutPrefix(path, prefix); under {
			leaf, _, _ := strings.Cut(rest, "/")
			leaves = append(leaves, leaf)
		}
	}
	return leaves
}

// TestTheGateFoldersAreTheLayoutsLeaves keeps the guards below from passing for want
// of anything to read: every gate still has its folder with Go files, and each
// folder that does, leaves added later included, keeps its settings in constants.go
// and says what it holds in doc.go, as every package of the layout does.
func TestTheGateFoldersAreTheLayoutsLeaves(t *testing.T) {
	folders := map[string][]string{}
	for _, f := range layoutFiles(t) {
		if f.leaf != "" {
			folders[f.leaf] = append(folders[f.leaf], f.path)
		}
	}
	for _, leaf := range layoutLeaves {
		if len(folders[leaf]) == 0 {
			t.Errorf("%s holds no Go file: the gate left the layout", leaf)
		}
	}
	for _, leaf := range slices.Sorted(maps.Keys(folders)) {
		for _, name := range []string{"doc.go", "constants.go"} {
			if !slices.Contains(folders[leaf], leaf+"/"+name) {
				t.Errorf("%s has no %s", leaf, name)
			}
		}
	}
}

// TestTheVocabularyImportsNoLeaf is the cycle rule the layout rests on. The compiler
// refuses the vocabulary's own files to import a leaf, which imports it back; it
// never sees an external test package of the vocabulary doing it, nor a file the
// build constraints leave out, and both are read here.
func TestTheVocabularyImportsNoLeaf(t *testing.T) {
	for _, f := range layoutFiles(t) {
		if f.leaf == "" && len(f.leafImports()) > 0 {
			t.Errorf("%s imports %v: the vocabulary is imported by the gates and imports none of them", f.path, f.leafImports())
		}
	}
}

// TestNoLeafImportsASibling keeps the gates independent of one another: a gate shares
// the vocabulary and nothing else. The one exception is a test of the depth priority
// gate, which builds a depth spacing hold row to prove no other row opens with the
// depth priority marker.
func TestNoLeafImportsASibling(t *testing.T) {
	for _, f := range layoutFiles(t) {
		for _, imported := range f.leafImports() {
			testOfDepthPriority := f.leaf == "depthpriority" && strings.HasSuffix(f.path, "_test.go")
			switch {
			case f.leaf == "", imported == f.leaf:
			case testOfDepthPriority && imported == "depthspacing":
			default:
				t.Errorf("%s imports the %s gate: a gate imports the flag's vocabulary and never another gate", f.path, imported)
			}
		}
	}
}

// TestTheVocabularyIsNamesAndEventsOnly pins what the vocabulary holds. Its constants
// are text, the names and the byte-stable row text of the gates; a number, a duration
// or a switch is a setting, and a setting is its gate's, in the gate's constants.go.
// It declares no variable and no function but the event constructors (a method of an
// event document is the document's): the holds are the gates'.
func TestTheVocabularyIsNamesAndEventsOnly(t *testing.T) {
	notText := func(expr ast.Expr) bool {
		lit, ok := expr.(*ast.BasicLit)
		return !ok || lit.Kind != token.STRING
	}
	for _, f := range layoutFiles(t) {
		if f.leaf != "" || strings.HasSuffix(f.path, "_test.go") {
			continue
		}
		for _, decl := range f.tree.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				isConstructor := strings.HasPrefix(d.Name.Name, "New") && strings.HasSuffix(d.Name.Name, "Event")
				if d.Recv == nil && !isConstructor {
					t.Errorf("%s declares func %s: a hold belongs to its gate's folder", f.path, d.Name.Name)
				}
			case *ast.GenDecl:
				if d.Tok == token.VAR {
					t.Errorf("%s declares a var: a setting is its gate's, never global to the flag", f.path)
				}
				for _, spec := range d.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || d.Tok != token.CONST {
						continue
					}
					if len(value.Values) != len(value.Names) || slices.ContainsFunc(value.Values, notText) {
						t.Errorf("%s declares %s, a constant that is not text: a setting is its gate's, never global to the flag", f.path, value.Names[0].Name)
					}
				}
			}
		}
	}
}
