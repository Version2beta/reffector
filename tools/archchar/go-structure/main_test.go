package main

import (
	"strings"
	"testing"
)

func TestExtractsTypesInterfacesMethodsAndSourceLocations(t *testing.T) {
	text := "package cache\n\nimport \"sync\"\n\n// Store owns state.\ntype Store struct {\n mu sync.Mutex\n}\n\ntype Reader interface {\n Read() error\n}\n\nfunc (s *Store) Open() error { return nil }\nfunc Public() {}\nconst DefaultMode = 1\n"
	result := run(input{Files: []sourceFile{{Package: "example.org/cache", Path: "cache.go", Text: text}}})
	if result.Status != "complete" {
		t.Fatalf("status %q: %v", result.Status, result.Errors)
	}
	if len(result.Files) != 1 || result.Files[0].PreambleEnd != 3 || result.Files[0].LineCount != len(strings.Split(text, "\n")) {
		t.Fatalf("unexpected file index: %+v", result.Files)
	}
	bySymbol := map[string]declaration{}
	for _, declaration := range result.Declarations {
		bySymbol[declaration.Symbol] = declaration
	}
	store, ok := bySymbol["Store"]
	if !ok || store.Kind != "type" || store.TypeKind != "struct" || len(store.Fields) != 1 || store.Fields[0].Names[0] != "mu" {
		t.Fatalf("store type was not extracted: %+v", store)
	}
	reader, ok := bySymbol["Reader"]
	if !ok || reader.TypeKind != "interface" || len(reader.InterfaceMethods) != 1 || reader.InterfaceMethods[0] != "Read() error" {
		t.Fatalf("interface method set was not extracted: %+v", reader)
	}
	open, ok := bySymbol["(*Store).Open"]
	if !ok || open.ReceiverType != "Store" || !open.ReceiverPointer || !open.HasBody || open.BodyStartLine == 0 || open.BodyEndLine < open.BodyStartLine {
		t.Fatalf("method metadata was not extracted: %+v", open)
	}
	public, ok := bySymbol["Public"]
	if !ok || !public.Exported || public.Kind != "function" {
		t.Fatalf("exported entry point was not extracted: %+v", public)
	}
	if _, ok := bySymbol["DefaultMode"]; !ok {
		t.Fatal("package constant was not included in structural inventory")
	}
}

func TestSyntaxErrorsAreReportedWithoutPartialSuccess(t *testing.T) {
	result := run(input{Files: []sourceFile{{Package: "example.org/bad", Path: "bad.go", Text: "package bad\nfunc {"}}})
	if result.Status != "partial" || len(result.Errors) != 1 {
		t.Fatalf("expected a parse error, got %+v", result)
	}
}
