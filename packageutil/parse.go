package packageutil

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

func ParsePackage(fset *token.FileSet, importPath, dir string) ([]*ast.File, error) {
	cfg := &packages.Config{
        Mode: packages.NeedName | packages.NeedImports | packages.NeedFiles,
        Dir:  dir,
    }
    pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("failed to load packages. importPath: %s, dir:%s", importPath, dir)
	}

	buildPkg, err := build.ImportDir(pkgs[0].Dir, build.IgnoreVendor)
	if err != nil {
		return nil, err
	}

	asts := make([]*ast.File, 0, len(buildPkg.GoFiles))
	for _, f := range buildPkg.GoFiles {
		filename := filepath.Join(buildPkg.Dir, f)
		ast, err := parser.ParseFile(fset, filename, nil, 0)
		if err != nil {
			return nil, err
		}
		asts = append(asts, ast)
	}
	return asts, nil
}
