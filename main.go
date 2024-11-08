package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"strings"
	"text/template"
)

//go:embed tmpl.gohtml
var tmpl string

func main() {
	var serviceName string
	var target string
	var imports string
	flag.StringVar(&serviceName, "service", "", "service name")
	flag.StringVar(&target, "target", "", "target")
	flag.StringVar(&imports, "imports", "", "imports")
	flag.Parse()
	if serviceName == "" {
		panic("service name is empty")
	}
	data := getData(serviceName)
	if imports != "" {
		data.Imports = strings.Split(imports, ",")
	}
	if data == nil {
		panic("service not found")
	}
	t := template.Must(template.New("tmpl").Funcs(
		map[string]any{
			"Add": func(v ...int) int {
				var sum int
				for _, vv := range v {
					sum += vv
				}
				return sum
			},
			"PrintExpr": func(v ast.Expr) string {
				return types.ExprString(v)
			},
		}).Parse(tmpl))
	var bb bytes.Buffer
	err := t.Execute(&bb, data)
	if err != nil {
		panic(err)
	}
	source, err := format.Source(bb.Bytes())
	if err != nil {
		source = bb.Bytes()
	} else {
		s := bufio.NewScanner(bytes.NewReader(source))
		var removeEmptyLine bytes.Buffer
		for s.Scan() {
			line := s.Bytes()
			if len(line) == 0 {
				continue
			}
			removeEmptyLine.Write(line)
			removeEmptyLine.WriteByte('\n')
		}
		source = removeEmptyLine.Bytes()
	}
	if target == "" {
		target = serviceName + "_gen.go"
	}
	err = os.WriteFile(target, source, 0644)
	if err != nil {
		panic(err)
	}
}

type Data struct {
	Package        string
	Service        string
	Client         string
	ServiceMethods []*Method
	ClientMethods  []*Method
	ClientID       string
	Imports        []string
}

func getData(serviceName string) *Data {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !info.IsDir() && !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	pkgName, serviceMethods := getMethods(packages, serviceName)
	var containsClose bool
	var clientIDType, removedClientIDType, clientType string
	for _, method := range serviceMethods {
		switch method.Name {
		case "AddClient":
			if len(method.Param) != 2 {
				panic("should have 2 params for AddClient(id ID_TYPE, client CLIENT_TYPE)")
			}
			if len(method.Result) != 0 {
				panic("should not have result for AddClient(id ID_TYPE, client CLIENT_TYPE)")
			}
			clientIDType = method.Param[0]
			clientType = method.Param[1]
		case "RemoveClient":
			if len(method.Param) != 1 {
				panic("should have 1 params for RemoveClient(id ID_TYPE)")
			}
			if len(method.Result) != 0 {
				panic("should not have result for RemoveClient(id ID_TYPE)")
			}
			removedClientIDType = method.Param[0]
		case "Close":
			if len(method.Param) != 0 {
				panic("should not have params for Close()")
			}
			if len(method.Result) != 0 {
				panic("should not have result for Close()")
			}
			containsClose = true
		}
	}
	if !containsClose {
		panic("service should have Close() method")
	}
	if clientIDType != removedClientIDType {
		panic("AddClient(id ID_TYPE, client CLIENT_TYPE) and RemoveClient(id ID_TYPE) should have the same ID_TYPE type")
	}
	var clientMethods []*Method
	if clientIDType != "" {
		var clientPkgName string
		clientPkgName, clientMethods = getMethods(packages, clientType)
		if pkgName != clientPkgName {
			panic("service and client should be in the same package")
		}
	}
	return &Data{
		Package:        pkgName,
		Service:        serviceName,
		Client:         clientType,
		ClientID:       clientIDType,
		ServiceMethods: serviceMethods,
		ClientMethods:  clientMethods,
	}
}
func getMethods(packages map[string]*ast.Package, name string) (pkgName string, methods []*Method) {
	for packageName, a := range packages {
		for _, file := range a.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typeSpec.Name.Name != name {
						continue
					}
					interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
					if !ok {
						panic("Handler should be interface")
					}
					var methods []*Method
					for _, field := range interfaceType.Methods.List {
						funcType, ok := field.Type.(*ast.FuncType)
						if !ok {
							panic("Handler method should be func")
						}
						name := field.Names[0].Name
						if ast.IsExported(name) {
							methods = append(methods, &Method{
								Name:   name,
								Param:  flattenFields(funcType.Params),
								Result: flattenFields(funcType.Results),
							})
						}
					}
					return packageName, methods
				}
			}
		}
	}
	return "", nil
}

type Method struct {
	Name   string
	Param  []string
	Result []string
}

func (m *Method) HasReturnedErr() bool {
	if len(m.Result) > 0 {
		return m.Result[len(m.Result)-1] == "error"
	}
	return false
}

func flattenFields(fl *ast.FieldList) []string {
	if fl == nil {
		return nil
	}
	var res []string
	for _, field := range fl.List {
		for range max(1, len(field.Names)) {
			res = append(res, types.ExprString(field.Type))
		}
	}
	return res
}
