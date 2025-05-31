package gocpbundle

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/matumoto1234/gocpbundle/packageutil"
	"golang.org/x/tools/go/types/typeutil"
)

var _ types.ImporterFrom = (*localImporter)(nil)

type localImporter struct {
	// cache はpath,dirに対するキャッシュ
	// pathとdirをこの順に.区切りで連結したものがキーに入る
	cache map[string]*types.Package

	fset *token.FileSet

	typesInfo *types.Info

	// 型解析中に読み込んだASTファイル一覧
	astFiles []*ast.File
}

func NewLocalImporter(fset *token.FileSet, typesInfo *types.Info) *localImporter {
	return &localImporter{
		cache:     map[string]*types.Package{},
		fset:      fset,
		typesInfo: typesInfo,
	}
}

func cacheKey(path, dir string) string {
	return strings.Join([]string{path, dir}, ".")
}

// Importer is present for backward-compatibility. Calling
// Import(path) is the same as calling ImportFrom(path, "", 0);
// i.e., locally vendored packages may not be found.
// The types package does not call Import if an ImporterFrom
// is present.
func (i *localImporter) Import(path string) (*types.Package, error) {
	return i.ImportFrom(path, ".", 0)
}

// ImportFrom returns the imported package for the given import
// path when imported by a package file located in dir.
// If the import failed, besides returning an error, ImportFrom
// is encouraged to cache and return a package anyway, if one
// was created. This will reduce package inconsistencies and
// follow-on type checker errors due to the missing package.
// The mode value must be 0; it is reserved for future use.
// Two calls to ImportFrom with the same path and dir must
// return the same package.
func (i *localImporter) ImportFrom(path, dir string, _ types.ImportMode) (*types.Package, error) {
	// すでにキャッシュできている場合は、それを返す
	key := cacheKey(path, dir)
	if typesPkg, ok := i.cache[key]; ok {
		return typesPkg, nil
	}

	asts, err := packageutil.ParsePackage(i.fset, path, dir)
	if err != nil {
		return nil, err
	}

	i.astFiles = append(i.astFiles, asts...)

	typesPkg, err := i.typeCheck(path, asts)
	if err != nil {
		return nil, err
	}

	i.cache[key] = typesPkg

	return typesPkg, nil
}

func (i *localImporter) typeCheck(importPath string, astFiles []*ast.File) (*types.Package, error) {
	// unsafeの場合、型チェックを行えないためtypes.Unsafeを返す
	if importPath == "unsafe" {
		return types.Unsafe, nil
	}

	typesConfig := &types.Config{
		IgnoreFuncBodies: packageutil.IsStandardPackageName(importPath),
		Importer:         i,
	}
	return typesConfig.Check(importPath, i.fset, astFiles, i.typesInfo)
}

func Bundle(filePath string) error {
	fset := token.NewFileSet()

	filePath, err := toAbsolutePath(filePath)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return err
	}

	typesInfo := &types.Info{
		Types:        map[ast.Expr]types.TypeAndValue{},
		Instances:    map[*ast.Ident]types.Instance{},
		Defs:         map[*ast.Ident]types.Object{},
		Uses:         map[*ast.Ident]types.Object{},
		Implicits:    map[ast.Node]types.Object{},
		Selections:   map[*ast.SelectorExpr]*types.Selection{},
		Scopes:       map[ast.Node]*types.Scope{},
		InitOrder:    []*types.Initializer{},
		FileVersions: map[*ast.File]string{},
	}

	localImporter := NewLocalImporter(fset, typesInfo)

	// types.Configはimporterを必要とし、importerはtypes.Configを必要とするため相互再帰になる(; ;)
	typesConfig := types.Config{
		Importer: localImporter,
	}

	// importerが内部で呼び出されるため、再帰的に型チェック
	_, err = typesConfig.Check(filePath, fset, []*ast.File{f}, typesInfo)
	if err != nil {
		return err
	}

	// types.Object.Id() -> ast.Decl
	topLevelDeclMap := constructTopLevelDeclMap(localImporter.astFiles, localImporter.typesInfo)

	appendingDecls := make([]ast.Decl, 0)

	var dfsAppendDecl func(ast.Node) bool
	dfsAppendDecl = func(n ast.Node) bool {
		switch n := n.(type) {
		// TODO: メソッドや型のDeclも持ってくる

		case *ast.CallExpr:
			obj := typeutil.Callee(localImporter.typesInfo, n)
			if obj == nil {
				break
			}

			decl, ok := topLevelDeclMap[obj.Id()]
			if decl == nil || !ok {
				break
			}

			ast.Inspect(decl, dfsAppendDecl)

			appendingDecls = append(appendingDecls, decl)
		}

		return true

	}

	// main.goのASTを見て呼び出しごとに定義をスライスに追加
	ast.Inspect(f, dfsAppendDecl)

	// main.goのASTに定義追加
	f.Decls = append(f.Decls, appendingDecls...)

	// ast.Print(localImporter.fset, f)

	err = format.Node(os.Stdout, localImporter.fset, f)
	if err != nil {
		return err
	}

	// TODO: addutil.Add() のような不要なセレクターを削除する(参考: https://github.com/ktateish/gottani/blob/master/internal/appinfo/squash.go#L553)
	// TODO: importも削除（goimportsコマンドをExecするだけでいいかもだけど）
	// TODO: importを追加（Add[cmp.Ordered]のように持ってきた宣言の中で標準パッケージを使ってるかも）

	// fmt.Printf("Defs: %+v\n", localImporter.typesInfo.Defs)
	// fmt.Printf("Uses: %+v\n", localImporter.typesInfo.Uses)
	// fmt.Printf("Selections: %+v\n", localImporter.typesInfo.Selections)
	// fmt.Printf("Types: %+v\n", localImporter.typesInfo.Types)
	// fmt.Printf("Instances: %+v\n", localImporter.typesInfo.Instances)

	return nil
}

// types.Id() -> ast.Decl なるマップを返す
// *ast.GenDecl | *ast.FundDecl のいずれか
// トップレベルの宣言のみ（関数内での宣言は入らない）
func constructTopLevelDeclMap(astFiles []*ast.File, typesInfo *types.Info) map[string]ast.Decl {
	topLevelDeclMap := make(map[string]ast.Decl)

	for _, f := range astFiles {
		for _, d := range f.Decls {
			switch d.(type) {
			case *ast.FuncDecl:
				funcDecl := d.(*ast.FuncDecl)

				typesObj := typesInfo.ObjectOf(funcDecl.Name)

				if typesObj == nil {
					fmt.Println("FuncDecl:", funcDecl.Name)
					break
				}

				if packageutil.IsStandardPackage(typesObj.Pkg()) {
					break
				}

				topLevelDeclMap[typesObj.Id()] = funcDecl
			case *ast.GenDecl:
				genDecl := d.(*ast.GenDecl)

				singleSpecGenDecls := splitGenDecl(genDecl)

				for _, d := range singleSpecGenDecls {
					s := d.Specs[0]

					switch s.(type) {
					case *ast.TypeSpec:
						typeSpec := s.(*ast.TypeSpec)
						typesObj := typesInfo.ObjectOf(typeSpec.Name)

						if typesObj == nil {
							fmt.Println("TypeSpec:", typeSpec.Name)
							break
						}

						if packageutil.IsStandardPackage(typesObj.Pkg()) {
							break
						}

						topLevelDeclMap[typesObj.Id()] = genDecl
					case *ast.ValueSpec:
						// めんどいのでValueSpecは分割しない(var a, b = 1, 2は var a = 1, var b = 1に分割しない)
						// 欲しくなったら実装
						valueSpec := s.(*ast.ValueSpec)
						for _, name := range valueSpec.Names {
							typesObj := typesInfo.ObjectOf(name)

							if typesObj == nil {
								fmt.Println("ValueSpec:", name)
								continue
							}

							if packageutil.IsStandardPackage(typesObj.Pkg()) {
								break
							}

							topLevelDeclMap[typesObj.Id()] = genDecl
						}
					case *ast.ImportSpec:
						importSpec := s.(*ast.ImportSpec)
						typesObj := typesInfo.ObjectOf(importSpec.Name)

						if typesObj == nil {
							fmt.Println("ImportSpec:", importSpec.Name, "path:", importSpec.Path)
							continue
						}

						topLevelDeclMap[typesObj.Id()] = genDecl
					}

				}
			}
		}
	}

	return topLevelDeclMap
}

// (*ast.GenDecl).Specsを一つ一つの*ast.GenDeclに分割する
// e.g. var ( a int, b int ) のようなものを var a int, var b int にする
func splitGenDecl(decl *ast.GenDecl) []*ast.GenDecl {
	var result []*ast.GenDecl

	for _, spec := range decl.Specs {
		// 新しい GenDecl を作る（位置情報などもコピー）
		newDecl := &ast.GenDecl{
			TokPos: decl.TokPos,
			Tok:    decl.Tok,
			Lparen: token.NoPos, // 分割後は () ブロックでないので Lparen, Rparen は無効に
			Rparen: token.NoPos,
			Specs:  []ast.Spec{spec},
			Doc:    decl.Doc, // Docを共有(TODO: 展開時にDocを共有するかどうかはCLIのオプションにしたい)
		}
		result = append(result, newDecl)
	}

	return result
}

func toAbsolutePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}
