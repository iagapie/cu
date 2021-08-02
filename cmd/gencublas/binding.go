package main

// binding.go is copied from gonum/internal and provides helpers for building autogenerated cgo bindings.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"
	"text/template"

	bg "github.com/gorgonia/bindgen"
	"modernc.org/cc"
	"modernc.org/xc"
)

var goTypes = map[bg.TypeKey]bg.Template{
	{Kind: cc.Undefined}: bg.Pure("<undefined>"),
	{Kind: cc.Int}:       bg.Pure("int"),
	{Kind: cc.Float}:     bg.Pure("float32"),
	{Kind: cc.Float, IsPointer: true}: bg.Pure(template.Must(template.New("[]float32").Parse(
		`{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}float32{{else}}[]float32{{end}}`))),

	{Kind: cc.Double}: bg.Pure("float64"),
	{Kind: cc.Double, IsPointer: true}: bg.Pure(template.Must(template.New("[]float64").Parse(
		`{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}float64{{else}}[]float64{{end}}`))),
	{Kind: cc.Bool}:          bg.Pure("bool"),
	{Kind: cc.FloatComplex}:  bg.Pure("complex64"),
	{Kind: cc.DoubleComplex}: bg.Pure("complex128"),

	{Kind: cc.FloatComplex, IsPointer: true}: bg.Pure(template.Must(template.New("cuComplex*").Parse(
		`{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}complex64{{else}}[]complex64{{end}}`,
	))),
	{Kind: cc.DoubleComplex, IsPointer: true}: bg.Pure(template.Must(template.New("cuDoubleComplex*").Parse(
		`{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}complex128{{else}}[]complex128{{end}}`,
	))),
	{Kind: cc.Int, IsPointer: true}: bg.Pure(template.Must(template.New("int*").Parse(
		`{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}int{{else}}[]int{{end}}`))),
}

// GoTypeFor returns a string representation of the given type using a mapping in
// types. GoTypeFor will panic if no type mapping is found after searching the
// user-provided types mappings and then the following mapping:
//  {Kind: cc.Int}:                     "int",
//  {Kind: cc.Float}:                   "float32",
//  {Kind: cc.Float, IsPointer: true}:  "[]float32",
//  {Kind: cc.Double}:                  "float64",
//  {Kind: cc.Double, IsPointer: true}: "[]float64",
//  {Kind: cc.Bool}:                    "bool",
//  {Kind: cc.FloatComplex}:            "complex64",
//  {Kind: cc.DoubleComplex}:           "complex128",
func GoTypeFor(typ cc.Type, name string, types ...map[bg.TypeKey]bg.Template) string {
	if typ == nil {
		return "<nil>"
	}
	k := typ.Kind()
	isPtr := typ.Kind() == cc.Ptr
	if isPtr {
		k = typ.Element().Kind()
	}
	var buf bytes.Buffer
	for _, t := range types {
		if s, ok := t[bg.TypeKey{Kind: k, IsPointer: isPtr}]; ok {
			err := s.Execute(&buf, name)
			if err != nil {
				panic(err)
			}
			return buf.String()
		}
	}
	s, ok := goTypes[bg.TypeKey{Kind: k, IsPointer: isPtr}]
	if ok {
		err := s.Execute(&buf, name)
		if err != nil {
			panic(err)
		}
		return buf.String()
	}
	log.Printf("%v", typ.Tag())
	panic(fmt.Sprintf("unknown type key: %v %+v", typ, bg.TypeKey{Kind: k, IsPointer: isPtr}))
}

// GoTypeForEnum returns a string representation of the given enum type using a mapping
// in types. GoTypeForEnum will panic if no type mapping is found after searching the
// user-provided types mappings or the type is not an enum.
func GoTypeForEnum(typ cc.Type, name string, types ...map[string]bg.Template) string {
	if typ == nil {
		return "<nil>"
	}
	if typ.Kind() != cc.Enum {
		panic(fmt.Sprintf("invalid type: %v", typ))
	}
	tag := typ.Tag()
	if tag != 0 {
		n := string(xc.Dict.S(tag))
		for _, t := range types {
			if s, ok := t[n]; ok {
				var buf bytes.Buffer
				err := s.Execute(&buf, name)
				if err != nil {
					panic(err)
				}
				return buf.String()
			}
		}
	}
	log.Printf("%s", typ.Declarator())
	panic(fmt.Sprintf("unknown type: %+v", typ))
}

var cgoTypes = map[bg.TypeKey]bg.Template{
	{Kind: cc.Void, IsPointer: true}: bg.Pure(template.Must(template.New("void*").Parse("unsafe.Pointer(&{{.}}[0])"))),

	{Kind: cc.Int}: bg.Pure(template.Must(template.New("int").Parse("C.int({{.}})"))),

	{Kind: cc.Float}:  bg.Pure(template.Must((template.New("float").Parse("C.float({{.}})")))),
	{Kind: cc.Double}: bg.Pure(template.Must((template.New("double").Parse("C.double({{.}})")))),

	{Kind: cc.Float, IsPointer: true}: bg.Pure(template.Must(template.New("float*").Parse(
		`(*C.float)(&{{.}}{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}{{else}}[0]{{end}})`))),
	{Kind: cc.Double, IsPointer: true}: bg.Pure(template.Must(template.New("double*").Parse(
		`(*C.double)(&{{.}}{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}{{else}}[0]{{end}})`))),

	{Kind: cc.Bool}: bg.Pure(template.Must((template.New("bool").Parse("C.bool({{.}})")))),

	{Kind: cc.FloatComplex}: bg.Pure(template.Must(template.New("floatcomplex").Parse(
		`*(*C.cuComplex)(unsafe.Pointer({{.}}))`))),
	{Kind: cc.DoubleComplex}: bg.Pure(template.Must(template.New("doublecomplex").Parse(
		`*(*C.cuDoubleComplex)(unsafe.Pointer({{.}}))`))),

	{Kind: cc.FloatComplex, IsPointer: true}: bg.Pure(template.Must(template.New("floatcomplex*").Parse(
		`(*C.cuComplex)(unsafe.Pointer(&{{.}}{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}{{else}}[0]{{end}}))`))),
	{Kind: cc.DoubleComplex, IsPointer: true}: bg.Pure(template.Must(template.New("doublecomplex*").Parse(
		`(*C.cuDoubleComplex)(unsafe.Pointer(&{{.}}{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}{{else}}[0]{{end}}))`))),

	// from main.go
	{Kind: cc.Void, IsPointer: true}: bg.Pure(template.Must(template.New("void*").Parse(
		`unsafe.Pointer(&{{.}}{{if eq . "alpha" "beta"}}{{else}}[0]{{end}})`,
	))),
	{Kind: cc.Int, IsPointer: true}: bg.Pure(template.Must(template.New("int*").Parse(
		`(*C.int)(&{{.}}{{if eq . "alpha" "beta" "cScalar" "sScalar" "result" "retVal"}}{{else}}[0]{{end}})`))),
}

// CgoConversionFor returns a string representation of the given type using a mapping in
// types. CgoConversionFor will panic if no type mapping is found after searching the
// user-provided types mappings and then the following mapping:
//  {Kind: cc.Void, IsPointer: true}:          "unsafe.Pointer(&{{.}}[0])",
//  {Kind: cc.Int}:                            "C.int({{.}})",
//  {Kind: cc.Float}:                          "C.float({{.}})",
//  {Kind: cc.Float, IsPointer: true}:         "(*C.float)({{.}})",
//  {Kind: cc.Double}:                         "C.double({{.}})",
//  {Kind: cc.Double, IsPointer: true}:        "(*C.double)({{.}})",
//  {Kind: cc.Bool}:                           "C.bool({{.}})",
//  {Kind: cc.FloatComplex}:                   "unsafe.Pointer(&{{.}})",
//  {Kind: cc.DoubleComplex}:                  "unsafe.Pointer(&{{.}})",
//  {Kind: cc.FloatComplex, IsPointer: true}:  "unsafe.Pointer(&{{.}}[0])",
//  {Kind: cc.DoubleComplex, IsPointer: true}: "unsafe.Pointer(&{{.}}[0])",
func CgoConversionFor(name string, typ cc.Type, types ...map[bg.TypeKey]bg.Template) string {
	if typ == nil {
		return "<nil>"
	}
	k := typ.Kind()
	isPtr := typ.Kind() == cc.Ptr
	if isPtr {
		k = typ.Element().Kind()
	}
	for _, t := range types {
		if s, ok := t[bg.TypeKey{Kind: k, IsPointer: isPtr}]; ok {
			var buf bytes.Buffer
			err := s.Execute(&buf, name)
			if err != nil {
				panic(err)
			}
			return buf.String()
		}
	}
	s, ok := cgoTypes[bg.TypeKey{Kind: k, IsPointer: isPtr}]
	if ok {
		var buf bytes.Buffer
		err := s.Execute(&buf, name)
		if err != nil {
			panic(err)
		}
		return buf.String()
	}
	panic(fmt.Sprintf("unknown type key: %+v", bg.TypeKey{Kind: k, IsPointer: isPtr}))
}

// CgoConversionForEnum returns a string representation of the given enum type using a mapping
// in types. GoTypeForEnum will panic if no type mapping is found after searching the
// user-provided types mappings or the type is not an enum.
func CgoConversionForEnum(name string, typ cc.Type, types ...map[string]bg.Template) string {
	if typ == nil {
		return "<nil>"
	}
	if typ.Kind() != cc.Enum {
		panic(fmt.Sprintf("invalid type: %v", typ))
	}
	tag := typ.Tag()
	if tag != 0 {
		n := string(xc.Dict.S(tag))
		for _, t := range types {
			if s, ok := t[n]; ok {
				var buf bytes.Buffer
				err := s.Execute(&buf, name)
				if err != nil {
					panic(err)
				}
				return buf.String()
			}
		}
	}
	panic(fmt.Sprintf("unknown type: %+v", typ))
}

// LowerCaseFirst returns s with the first character lower-cased. LowerCaseFirst
// assumes s is an ASCII-represented string.
func LowerCaseFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]|' ') + s[1:]
}

// UpperCaseFirst returns s with the first character upper-cased. UpperCaseFirst
// assumes s is an ASCII-represented string.
func UpperCaseFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]&^' ') + s[1:]
}

// DocComments returns a map of method documentation comments for the package at the
// given path. The first key of the returned map is the type name and the second
// is the method name. Non-method function documentation are in docs[""].
func DocComments(path string) (docs map[string]map[string][]*ast.Comment, err error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	docs = make(map[string]map[string][]*ast.Comment)
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, n := range f.Decls {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Doc == nil {
					continue
				}

				var typ string
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					id, ok := fn.Recv.List[0].Type.(*ast.Ident)
					if ok {
						typ = id.Name
					}
				}
				doc, ok := docs[typ]
				if !ok {
					doc = make(map[string][]*ast.Comment)
					docs[typ] = doc
				}
				doc[fn.Name.String()] = fn.Doc.List
			}
		}
	}
	return docs, nil
}

// functions say we only want functions declared
func functions(t *cc.TranslationUnit) ([]bg.Declaration, error) {
	filter := func(d *cc.Declarator) bool {
		if d.Type.Kind() != cc.Function {
			return false
		}
		return true
	}
	return bg.Get(t, filter)
}

func shorten(n string) string {
	s, ok := names[n]
	if ok {
		return s
	}
	return n
}

func cblasTocublas(name string) string {
	retVal := strings.TrimPrefix(name, prefix)
	return fmt.Sprintf("cublas%s", strings.Title(retVal))
}
