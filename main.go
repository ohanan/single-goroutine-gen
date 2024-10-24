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
	flag.StringVar(&serviceName, "service", "", "service name")
	flag.StringVar(&target, "target", "", "target")
	flag.Parse()
	if serviceName == "" {
		panic("service name is empty")
	}
	data := getData(serviceName)
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
		target = data.file + "_gen"
	}
	err = os.WriteFile(target, source, 0644)
	if err != nil {
		panic(err)
	}
}

type Data struct {
	file    string
	Package string
	Service string
	Methods []*Method
}

func getData(serviceName string) *Data {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !info.IsDir() && !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		panic(err)
	}
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
					if typeSpec.Name.Name != serviceName {
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
					return &Data{
						file:    file.Name.Name,
						Package: packageName,
						Service: serviceName,
						Methods: methods,
					}
				}
			}
		}
	}
	return nil
}

func writeFields(bb *bytes.Buffer, fields []*ast.Field) {
	for i, field := range fields {
		if i > 0 {
			bb.WriteString(", ")
		}
		for i, name := range field.Names {
			if i > 0 {
				bb.WriteString(", ")
			}
			bb.WriteString(name.Name)
		}
		bb.WriteString(" ")
		bb.WriteString(types.ExprString(field.Type))
	}
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

func getMethods() []*Method {
	f, err := parser.ParseFile(token.NewFileSet(), "proto.go", nil, parser.ParseComments)
	if err != nil {
		panic(f)
	}
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name
			if name != "HandlerInterface" {
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
				methods = append(methods, &Method{
					Name:   field.Names[0].Name,
					Param:  flattenFields(funcType.Params),
					Result: flattenFields(funcType.Results),
				})
			}
			return methods
		}
	}
	return nil
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
