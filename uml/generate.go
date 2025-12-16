package uml

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Generate(root string, opts Options) (*Model, error) {
	opts = opts.withDefaults()

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	rootInfo, err := os.Stat(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}

	baseDir := rootAbs
	onlyFile := ""
	if !rootInfo.IsDir() {
		baseDir = filepath.Dir(rootAbs)
		onlyFile = filepath.Base(rootAbs)
	}

	modLocator := newModuleLocator()
	rootModule, err := modLocator.moduleForDir(baseDir)
	if err != nil {
		return nil, err
	}

	model := &Model{
		GeneratedAt: time.Now().UTC(),
		Root:        rootAbs,
		Module:      rootModule,
	}

	fset := token.NewFileSet()
	var pkgs []Package

	if onlyFile != "" {
		pkgs, err = parseDir(fset, modLocator, baseDir, onlyFile, baseDir, opts)
		if err != nil {
			return nil, err
		}
	} else {
		excluded := make(map[string]struct{}, len(opts.ExcludeDirNames))
		for _, name := range opts.ExcludeDirNames {
			excluded[name] = struct{}{}
		}

		err = filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}

			if _, ok := excluded[entry.Name()]; ok {
				return fs.SkipDir
			}

			parsed, err := parseDir(fset, modLocator, path, "", baseDir, opts)
			if err != nil {
				if errors.Is(err, errNoGoFiles) {
					return nil
				}
				return err
			}
			pkgs = append(pkgs, parsed...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].ImportPath != pkgs[j].ImportPath {
			return pkgs[i].ImportPath < pkgs[j].ImportPath
		}
		if pkgs[i].Dir != pkgs[j].Dir {
			return pkgs[i].Dir < pkgs[j].Dir
		}
		return pkgs[i].Name < pkgs[j].Name
	})

	model.Packages = pkgs
	return model, nil
}

func GenerateJSON(root string, opts Options) ([]byte, error) {
	opts = opts.withDefaults()
	model, err := Generate(root, opts)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(model, "", opts.Indent)
}

var errNoGoFiles = errors.New("no go files")

func parseDir(fset *token.FileSet, modLocator *moduleLocator, dir, onlyFile, relBase string, opts Options) ([]Package, error) {
	filter := func(info fs.FileInfo) bool {
		name := info.Name()
		if !strings.HasSuffix(name, ".go") {
			return false
		}
		if onlyFile != "" && name != onlyFile {
			return false
		}
		if !opts.IncludeTests && strings.HasSuffix(name, "_test.go") {
			return false
		}
		if !opts.IncludeGenerated {
			isGen, err := isGeneratedFile(filepath.Join(dir, name))
			if err == nil && isGen {
				return false
			}
		}
		return true
	}

	parsed, err := parser.ParseDir(fset, dir, filter, parser.ParseComments)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errNoGoFiles
		}
		if strings.Contains(err.Error(), "no Go files") {
			return nil, errNoGoFiles
		}
		return nil, fmt.Errorf("parse dir %s: %w", dir, err)
	}
	if len(parsed) == 0 {
		return nil, errNoGoFiles
	}

	var out []Package
	for pkgName, pkg := range parsed {
		pkgModel, err := extractPackage(fset, modLocator, dir, pkgName, pkg, relBase)
		if err != nil {
			return nil, err
		}
		pkgModel = sortPackage(pkgModel)
		out = append(out, pkgModel)
	}
	return out, nil
}

func extractPackage(fset *token.FileSet, modLocator *moduleLocator, dir, pkgName string, pkg *ast.Package, relBase string) (Package, error) {
	pkgModel := Package{
		Name: pkgName,
		Dir:  toRelPath(relBase, dir),
	}

	mod, err := modLocator.moduleForDir(dir)
	if err != nil {
		return Package{}, err
	}
	if mod != nil {
		rel, err := filepath.Rel(mod.Dir, dir)
		if err == nil {
			if rel == "." {
				pkgModel.ImportPath = mod.Path
			} else {
				pkgModel.ImportPath = mod.Path + "/" + filepath.ToSlash(rel)
			}
		}
	}

	pkgModel.Files = packageFiles(relBase, dir, pkg)

	typesByName := map[string]int{}

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				t := Type{
					Name:       ts.Name.Name,
					Kind:       kindForTypeSpec(ts),
					Exported:   ast.IsExported(ts.Name.Name),
					Doc:        docText(ts.Doc, gen.Doc, len(gen.Specs) == 1),
					TypeParams: parseTypeParams(fset, ts.TypeParams),
				}
				switch st := ts.Type.(type) {
				case *ast.StructType:
					t.Fields, t.Embedded = parseStructFields(fset, st.Fields)
				case *ast.InterfaceType:
					t.Methods, t.Embedded = parseInterfaceMethods(fset, st.Methods)
				}
				pkgModel.Types = append(pkgModel.Types, t)
				typesByName[t.Name] = len(pkgModel.Types) - 1
			}
		}
	}

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			fnModel := Function{
				Name:       fn.Name.Name,
				Exported:   ast.IsExported(fn.Name.Name),
				Doc:        docText(fn.Doc, nil, false),
				TypeParams: parseTypeParams(fset, fn.Type.TypeParams),
			}
			fnModel.Params, fnModel.Variadic = parseParams(fset, fn.Type.Params)
			fnModel.Results = parseResults(fset, fn.Type.Results)

			if fn.Recv == nil || len(fn.Recv.List) == 0 {
				pkgModel.Functions = append(pkgModel.Functions, fnModel)
				continue
			}

			recvType := fn.Recv.List[0].Type
			fnModel.Receiver = exprString(fset, recvType)

			recvName := receiverBaseName(recvType)
			if recvName == "" {
				pkgModel.Functions = append(pkgModel.Functions, fnModel)
				continue
			}
			if idx, ok := typesByName[recvName]; ok {
				pkgModel.Types[idx].Methods = append(pkgModel.Types[idx].Methods, fnModel)
				continue
			}
			pkgModel.Functions = append(pkgModel.Functions, fnModel)
		}
	}

	return pkgModel, nil
}

func sortPackage(pkg Package) Package {
	sort.Strings(pkg.Files)

	sort.Slice(pkg.Functions, func(i, j int) bool {
		return pkg.Functions[i].Name < pkg.Functions[j].Name
	})

	sort.Slice(pkg.Types, func(i, j int) bool {
		return pkg.Types[i].Name < pkg.Types[j].Name
	})
	for i := range pkg.Types {
		sort.Slice(pkg.Types[i].Fields, func(a, b int) bool {
			return pkg.Types[i].Fields[a].Name < pkg.Types[i].Fields[b].Name
		})
		sort.Strings(pkg.Types[i].Embedded)
		sort.Slice(pkg.Types[i].Methods, func(a, b int) bool {
			return pkg.Types[i].Methods[a].Name < pkg.Types[i].Methods[b].Name
		})
	}

	return pkg
}

func parseStructFields(fset *token.FileSet, fields *ast.FieldList) ([]Field, []string) {
	if fields == nil || len(fields.List) == 0 {
		return nil, nil
	}

	var out []Field
	var embedded []string

	for _, field := range fields.List {
		typ := exprString(fset, field.Type)
		tag := ""
		if field.Tag != nil {
			tag = strings.Trim(field.Tag.Value, "`")
		}
		if len(field.Names) == 0 {
			embedded = append(embedded, typ)
			continue
		}
		for _, name := range field.Names {
			out = append(out, Field{
				Name:     name.Name,
				Type:     typ,
				Tag:      tag,
				Exported: ast.IsExported(name.Name),
			})
		}
	}

	return out, embedded
}

func parseInterfaceMethods(fset *token.FileSet, methods *ast.FieldList) ([]Function, []string) {
	if methods == nil || len(methods.List) == 0 {
		return nil, nil
	}

	var out []Function
	var embedded []string

	for _, field := range methods.List {
		if len(field.Names) == 0 {
			embedded = append(embedded, exprString(fset, field.Type))
			continue
		}
		if len(field.Names) != 1 {
			continue
		}

		name := field.Names[0].Name
		ft, ok := field.Type.(*ast.FuncType)
		if !ok {
			continue
		}

		fnModel := Function{
			Name:       name,
			Exported:   ast.IsExported(name),
			Doc:        docText(field.Doc, field.Comment, false),
			TypeParams: parseTypeParams(fset, ft.TypeParams),
		}
		fnModel.Params, fnModel.Variadic = parseParams(fset, ft.Params)
		fnModel.Results = parseResults(fset, ft.Results)
		out = append(out, fnModel)
	}

	return out, embedded
}

func kindForTypeSpec(ts *ast.TypeSpec) TypeKind {
	if ts.Assign.IsValid() {
		return TypeKindAlias
	}
	switch ts.Type.(type) {
	case *ast.StructType:
		return TypeKindStruct
	case *ast.InterfaceType:
		return TypeKindInterface
	default:
		return TypeKindOther
	}
}

func parseTypeParams(fset *token.FileSet, params *ast.FieldList) []TypeParam {
	if params == nil || len(params.List) == 0 {
		return nil
	}

	var out []TypeParam
	for _, field := range params.List {
		constraint := ""
		if field.Type != nil {
			constraint = exprString(fset, field.Type)
		}
		for _, name := range field.Names {
			out = append(out, TypeParam{
				Name:       name.Name,
				Constraint: constraint,
			})
		}
	}
	return out
}

func parseParams(fset *token.FileSet, params *ast.FieldList) ([]Param, bool) {
	if params == nil || len(params.List) == 0 {
		return nil, false
	}

	var out []Param
	var variadic bool

	for i, field := range params.List {
		typ := exprString(fset, field.Type)
		if i == len(params.List)-1 {
			_, variadic = field.Type.(*ast.Ellipsis)
		}
		if len(field.Names) == 0 {
			out = append(out, Param{Type: typ})
			continue
		}
		for _, name := range field.Names {
			out = append(out, Param{
				Name: name.Name,
				Type: typ,
			})
		}
	}

	return out, variadic
}

func parseResults(fset *token.FileSet, results *ast.FieldList) []Param {
	if results == nil || len(results.List) == 0 {
		return nil
	}

	var out []Param
	for _, field := range results.List {
		typ := exprString(fset, field.Type)
		if len(field.Names) == 0 {
			out = append(out, Param{Type: typ})
			continue
		}
		for _, name := range field.Names {
			out = append(out, Param{
				Name: name.Name,
				Type: typ,
			})
		}
	}
	return out
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return buf.String()
}

func receiverBaseName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return receiverBaseName(e.X)
	case *ast.IndexExpr:
		return receiverBaseName(e.X)
	case *ast.IndexListExpr:
		return receiverBaseName(e.X)
	case *ast.ParenExpr:
		return receiverBaseName(e.X)
	default:
		return ""
	}
}

func docText(primary, fallback *ast.CommentGroup, fallbackAllowed bool) string {
	if primary != nil {
		return strings.TrimSpace(primary.Text())
	}
	if fallbackAllowed && fallback != nil {
		return strings.TrimSpace(fallback.Text())
	}
	return ""
}

func toRelPath(baseDir, path string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func packageFiles(relBase, dir string, pkg *ast.Package) []string {
	files := make([]string, 0, len(pkg.Files))
	for file := range pkg.Files {
		files = append(files, toRelPath(relBase, filepath.Join(dir, file)))
	}
	sort.Strings(files)
	return files
}

func isGeneratedFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		line := scanner.Text()
		if strings.Contains(line, "DO NOT EDIT") && strings.Contains(line, "Code generated") {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

type moduleLocator struct {
	cache map[string]*Module
}

func newModuleLocator() *moduleLocator {
	return &moduleLocator{cache: map[string]*Module{}}
}

func (ml *moduleLocator) moduleForDir(dir string) (*Module, error) {
	dir = filepath.Clean(dir)
	if mod, ok := ml.cache[dir]; ok {
		return mod, nil
	}

	gomod := filepath.Join(dir, "go.mod")
	if fileExists(gomod) {
		modPath, err := readModulePath(gomod)
		if err != nil {
			return nil, err
		}
		mod := &Module{Path: modPath, Dir: dir}
		ml.cache[dir] = mod
		return mod, nil
	}

	parent := filepath.Dir(dir)
	if parent == dir {
		ml.cache[dir] = nil
		return nil, nil
	}

	mod, err := ml.moduleForDir(parent)
	if err != nil {
		return nil, err
	}
	ml.cache[dir] = mod
	return mod, nil
}

func readModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", goModPath, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
		if strings.HasPrefix(line, "module\t") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module\t")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", goModPath, err)
	}
	return "", fmt.Errorf("no module path in %s", goModPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
