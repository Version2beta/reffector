package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type input struct {
	Files []sourceFile `json:"files"`
}

type sourceFile struct {
	Package string `json:"package"`
	Path    string `json:"path"`
	Text    string `json:"text"`
}

type output struct {
	Status       string        `json:"status"`
	Files        []fileInfo    `json:"files"`
	Declarations []declaration `json:"declarations"`
	Errors       []string      `json:"errors"`
}

type fileInfo struct {
	Package        string   `json:"package"`
	Path           string   `json:"path"`
	PackageName    string   `json:"package_name"`
	Imports        []string `json:"imports"`
	PreambleEnd    int      `json:"preamble_end_line"`
	LineCount      int      `json:"line_count"`
	DeclarationIDs []string `json:"declaration_ids"`
}

type fieldInfo struct {
	Names    []string `json:"names"`
	Type     string   `json:"type"`
	Embedded bool     `json:"embedded"`
	Tag      string   `json:"tag,omitempty"`
	Start    int      `json:"start_line"`
	End      int      `json:"end_line"`
}

type declaration struct {
	ID               string      `json:"id"`
	Package          string      `json:"package"`
	File             string      `json:"file"`
	Kind             string      `json:"kind"`
	Name             string      `json:"name"`
	Symbol           string      `json:"symbol"`
	ReceiverType     string      `json:"receiver_type,omitempty"`
	ReceiverPointer  bool        `json:"receiver_pointer,omitempty"`
	TypeKind         string      `json:"type_kind,omitempty"`
	Signature        string      `json:"signature,omitempty"`
	TypeExpression   string      `json:"type_expression,omitempty"`
	Fields           []fieldInfo `json:"fields,omitempty"`
	InterfaceMethods []string    `json:"interface_methods,omitempty"`
	Exported         bool        `json:"exported"`
	HasBody          bool        `json:"has_body,omitempty"`
	StartLine        int         `json:"start_line"`
	EndLine          int         `json:"end_line"`
	BodyStartLine    int         `json:"body_start_line,omitempty"`
	BodyEndLine      int         `json:"body_end_line,omitempty"`
	Doc              string      `json:"doc,omitempty"`
}

func nodeText(fset *token.FileSet, node any) string {
	if node == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, node); err != nil {
		return ""
	}
	return buffer.String()
}

func line(fset *token.FileSet, pos token.Pos) int {
	if !pos.IsValid() {
		return 0
	}
	return fset.Position(pos).Line
}

func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

func receiverType(fset *token.FileSet, expr ast.Expr) (string, bool) {
	pointer := false
	for {
		switch value := expr.(type) {
		case *ast.StarExpr:
			pointer = true
			expr = value.X
		case *ast.ParenExpr:
			expr = value.X
		case *ast.IndexExpr:
			expr = value.X
		case *ast.IndexListExpr:
			expr = value.X
		default:
			return nodeText(fset, expr), pointer
		}
	}
}

func declarationID(pkg, path, kind, symbol string, start int) string {
	return fmt.Sprintf("%s::%s:%d:%s:%s", pkg, path, start, kind, symbol)
}

func addDecl(out *output, info *fileInfo, value declaration) {
	out.Declarations = append(out.Declarations, value)
	info.DeclarationIDs = append(info.DeclarationIDs, value.ID)
}

func run(in input) output {
	result := output{Status: "complete", Files: []fileInfo{}, Declarations: []declaration{}, Errors: []string{}}
	files := append([]sourceFile(nil), in.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, source := range files {
		if source.Package == "" || source.Path == "" {
			result.Errors = append(result.Errors, "each source file requires package and path")
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source.Path, source.Text, parser.ParseComments|parser.AllErrors)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", source.Path, err))
			continue
		}
		lines := strings.Split(source.Text, "\n")
		info := fileInfo{Package: source.Package, Path: source.Path, PackageName: file.Name.Name, LineCount: len(lines), PreambleEnd: line(fset, file.Package), Imports: []string{}, DeclarationIDs: []string{}}
		for _, declarationNode := range file.Decls {
			if importDecl, ok := declarationNode.(*ast.GenDecl); ok && importDecl.Tok == token.IMPORT {
				if end := line(fset, importDecl.End()); end > info.PreambleEnd {
					info.PreambleEnd = end
				}
			}
		}
		for _, importSpec := range file.Imports {
			if importPath, unquoteErr := strconv.Unquote(importSpec.Path.Value); unquoteErr == nil {
				info.Imports = append(info.Imports, importPath)
			}
			if end := line(fset, importSpec.End()); end > info.PreambleEnd {
				info.PreambleEnd = end
			}
		}
		for _, declarationNode := range file.Decls {
			switch node := declarationNode.(type) {
			case *ast.FuncDecl:
				receiver := ""
				pointer := false
				symbol := node.Name.Name
				if node.Recv != nil && len(node.Recv.List) > 0 {
					receiver, pointer = receiverType(fset, node.Recv.List[0].Type)
					symbol = "(" + receiver + ")." + node.Name.Name
					if pointer {
						symbol = "(*" + receiver + ")." + node.Name.Name
					}
				}
				start := line(fset, node.Pos())
				if node.Doc != nil {
					start = line(fset, node.Doc.Pos())
				}
				end := line(fset, node.End())
				decl := declaration{
					ID: declarationID(source.Package, source.Path, "function", symbol, start), Package: source.Package, File: source.Path,
					Kind: "function", Name: node.Name.Name, Symbol: symbol, ReceiverType: receiver, ReceiverPointer: pointer,
					Signature: nodeText(fset, node.Type), Exported: ast.IsExported(node.Name.Name), HasBody: node.Body != nil,
					StartLine: start, EndLine: end, Doc: commentText(node.Doc),
				}
				if node.Body != nil {
					decl.BodyStartLine, decl.BodyEndLine = line(fset, node.Body.Pos()), line(fset, node.Body.End())
				}
				addDecl(&result, &info, decl)
			case *ast.GenDecl:
				if node.Tok == token.IMPORT {
					continue
				}
				for _, spec := range node.Specs {
					switch value := spec.(type) {
					case *ast.TypeSpec:
						start := line(fset, value.Pos())
						if value.Doc != nil {
							start = line(fset, value.Doc.Pos())
						} else if node.Doc != nil {
							start = line(fset, node.Doc.Pos())
						}
						typeKind := "defined"
						var fields []fieldInfo
						var methods []string
						switch typeNode := value.Type.(type) {
						case *ast.StructType:
							typeKind = "struct"
							for _, field := range typeNode.Fields.List {
								names := make([]string, 0, len(field.Names))
								for _, name := range field.Names {
									names = append(names, name.Name)
								}
								tag := ""
								if field.Tag != nil {
									tag, _ = strconv.Unquote(field.Tag.Value)
								}
								fields = append(fields, fieldInfo{Names: names, Type: nodeText(fset, field.Type), Embedded: len(names) == 0, Tag: tag, Start: line(fset, field.Pos()), End: line(fset, field.End())})
							}
						case *ast.InterfaceType:
							typeKind = "interface"
							for _, field := range typeNode.Methods.List {
								if len(field.Names) == 0 {
									methods = append(methods, nodeText(fset, field.Type))
									continue
								}
								for _, method := range field.Names {
									signature := strings.TrimPrefix(nodeText(fset, field.Type), "func")
									methods = append(methods, method.Name+signature)
								}
							}
						}
						end := line(fset, value.End())
						addDecl(&result, &info, declaration{
							ID: declarationID(source.Package, source.Path, "type", value.Name.Name, start), Package: source.Package, File: source.Path,
							Kind: "type", Name: value.Name.Name, Symbol: value.Name.Name, TypeKind: typeKind,
							Signature: nodeText(fset, value.TypeParams), TypeExpression: nodeText(fset, value.Type), Fields: fields,
							InterfaceMethods: methods, Exported: ast.IsExported(value.Name.Name), StartLine: start, EndLine: end,
							Doc: commentText(value.Doc),
						})
					case *ast.ValueSpec:
						for _, name := range value.Names {
							start := line(fset, value.Pos())
							if value.Doc != nil {
								start = line(fset, value.Doc.Pos())
							} else if node.Doc != nil {
								start = line(fset, node.Doc.Pos())
							}
							addDecl(&result, &info, declaration{
								ID: declarationID(source.Package, source.Path, strings.ToLower(node.Tok.String()), name.Name, start), Package: source.Package, File: source.Path,
								Kind: strings.ToLower(node.Tok.String()), Name: name.Name, Symbol: name.Name, TypeExpression: nodeText(fset, value.Type),
								Exported: ast.IsExported(name.Name), StartLine: start, EndLine: line(fset, value.End()), Doc: commentText(value.Doc),
							})
						}
					}
				}
			}
		}
		sort.Strings(info.Imports)
		result.Files = append(result.Files, info)
	}
	if len(result.Errors) > 0 {
		result.Status = "partial"
	}
	sort.Slice(result.Declarations, func(i, j int) bool {
		a, b := result.Declarations[i], result.Declarations[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.ID < b.ID
	})
	return result
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read input:", err)
		os.Exit(2)
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		fmt.Fprintln(os.Stderr, "decode input:", err)
		os.Exit(2)
	}
	if len(in.Files) == 0 {
		fmt.Fprintln(os.Stderr, "input contains no source files")
		os.Exit(2)
	}
	result := run(in)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode output:", err)
		os.Exit(2)
	}
	if result.Status != "complete" {
		os.Exit(1)
	}
}
