// Command readability prints raw Go size metrics for scripts/readability-report.sh.
//
// For every non-generated, non-test Go file under the given directories it
// prints one line per file and one line per function:
//
//	FILE <lines> <path>
//	FUNC <body-lines> <name> <path>:<line>
//
// It uses only the standard library so it runs with `go run` outside any module.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var skipDirs = map[string]bool{
	".git":         true,
	"dist":         true,
	"node_modules": true,
	"testdata":     true,
	"vendor":       true,
}

// generatedPaths lists sqlc outputs whose headers are easy to miss when only
// some of a package is regenerated.
var generatedPaths = []string{
	"cometmind/internal/db/db.go",
	"cometmind/internal/db/models.go",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: readability <dir>...")
		os.Exit(2)
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for _, root := range os.Args[1:] {
		if err := walk(out, root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func walk(out *bufio.Writer, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isCandidate(path) {
			return nil
		}
		return measure(out, filepath.ToSlash(path))
	})
}

func isCandidate(path string) bool {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false
	}
	if strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".sql.go") {
		return false
	}
	slashed := filepath.ToSlash(path)
	for _, p := range generatedPaths {
		if strings.HasSuffix(slashed, p) {
			return false
		}
	}
	return true
}

func measure(out *bufio.Writer, path string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	if ast.IsGenerated(file) {
		return nil
	}
	fmt.Fprintf(out, "FILE %d %s\n", fset.File(file.Pos()).LineCount(), path)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		open := fset.Position(fn.Body.Lbrace).Line
		closing := fset.Position(fn.Body.Rbrace).Line
		lines := closing - open - 1
		if lines < 0 {
			lines = 0
		}
		fmt.Fprintf(out, "FUNC %d %s %s:%d\n", lines, funcName(file, fn), path, fset.Position(fn.Pos()).Line)
	}
	return nil
}

func funcName(file *ast.File, fn *ast.FuncDecl) string {
	name := file.Name.Name + "." + fn.Name.Name
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return name
	}
	return file.Name.Name + "." + receiverType(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.IndexExpr:
		return receiverType(t.X)
	case *ast.IndexListExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return "?"
	}
}
