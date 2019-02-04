package parse

import (
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"go/ast"
)

const letters = "abcdefghijklmnopqrstuvwxyz"

type field struct {
	fieldName string
	typeName  string
	funcName  string
	tagName   string
	omit      bool
	embedded  bool
	err       error
}

func (f field) getFieldName() string {
	if f.tagName != "" {
		return f.tagName
	}
	return f.fieldName
}

type Result struct {
	Fields []string
	Errors []error
}

// Fields gets the fields of the given struct.
// pth must be a go file that defines the typ struct.
// Any embedded structs must also be in that same file.
func Fields(typ, pth string) (*Result, error) {
	fullTyp := typ
	typ = getType(fullTyp)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, pth, nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	f := &finder{n: map[string]ast.Node{}}

	ast.Walk(visitorFunc(f.findTypes), file)

	if f.n == nil {
		return nil, fmt.Errorf("could not find %s", typ)
	}

	fields, err := doGetFields(f.n)
	if err != nil {
		return nil, err
	}

	var out []field
	var errs []error
	var i int
	for _, f := range fields[typ] {
		if f.err != nil {
			errs = append(errs, f.err)
		} else if f.err == nil && f.embedded {
			embeddedFields := fields[f.typeName]
			out = append(out[:i], append(embeddedFields, out[i:]...)...)
			i += len(embeddedFields)
		} else if f.err == nil {
			out = append(out, f)
			i++
		}
	}

	return &Result{
		Fields: formatFields(fullTyp, out),
		Errors: errs,
	}, nil
}

func getType(typ string) string {
	parts := strings.Split(typ, ".")
	return parts[len(parts)-1]
}

func formatFields(typ string, fields []field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !f.omit {
			out = append(out, fmt.Sprintf(`%s(func(x %s) %s { return x.%s }, func(x *%s, v %s) { x.%s = v }, "%s"),`, f.funcName, typ, f.typeName, f.fieldName, typ, f.typeName, f.fieldName, f.getFieldName()))
		}
	}
	return out
}

func isPrivate(x *ast.Field) bool {
	var s string
	if len(x.Names) == 0 {
		s = fmt.Sprintf("%s", x.Type)
	} else {
		s = fmt.Sprintf("%s", x.Names[0])
	}
	return strings.Contains(letters, string(s[0]))
}

func doGetFields(n map[string]ast.Node) (map[string][]field, error) {
	fields := map[string][]field{}
	for k, n := range n {
		ast.Inspect(n, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Field:
				if len(x.Names) == 1 && !isPrivate(x) {
					f := getField(x.Names[0].Name, x)
					fields[k] = append(fields[k], f)
				} else if len(x.Names) == 0 && !isPrivate(x) {
					fields[k] = append(fields[k], field{embedded: true, typeName: fmt.Sprintf("%s", x.Type)})
				}
			}
			return true
		})
	}
	return fields, nil
}

func getField(name string, x ast.Node) field {
	var typ, tag string
	var optional bool
	ast.Inspect(x, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.Field:
			if t.Tag != nil {
				tag = parseTag(t.Tag.Value)
			}
		case *ast.StarExpr:
			optional = true
		case ast.Expr:
			s := fmt.Sprintf("%v", t)
			_, ok := types[s]
			if ok {
				typ = s
			}
		}
		return true
	})

	var err error
	_, ok := types[typ]
	if !ok {
		err = fmt.Errorf("unsupported type: %s", name)
	}

	return field{fieldName: name, typeName: getTypeName(typ, optional), funcName: lookupType(typ, optional), tagName: tag, omit: tag == "-", err: err}
}

func parseTag(t string) string {
	i := strings.Index(t, `parquet:"`)
	if i == -1 {
		return ""
	}
	t = t[i+9:]
	return t[:strings.Index(t, `"`)]
}

func getTypeName(s string, optional bool) string {
	var star string
	if optional {
		star = "*"
	}
	return fmt.Sprintf("%s%s", star, s)
}

func lookupType(name string, optional bool) string {
	var op string
	if optional {
		op = "Optional"
	}
	f, ok := types[name]
	if !ok {
		return ""
	}
	return fmt.Sprintf(f, op)
}

var types = map[string]string{
	"int32":   "NewInt32%sField",
	"uint32":  "NewUint32%sField",
	"int64":   "NewInt64%sField",
	"uint64":  "NewUint64%sField",
	"float32": "NewFloat32%sField",
	"float64": "NewFloat64%sField",
	"bool":    "NewBool%sField",
	"string":  "NewString%sField",
}

type visitorFunc func(n ast.Node) ast.Visitor

func (f visitorFunc) Visit(n ast.Node) ast.Visitor {
	return f(n)
}

type finder struct {
	n map[string]ast.Node
}

func (f *finder) findTypes(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.ImportSpec:
		return visitorFunc(f.findTypes)
	case *ast.Package:
		return visitorFunc(f.findTypes)
	case *ast.File:
		return visitorFunc(f.findTypes)
	case *ast.GenDecl:
		if n.Tok == token.TYPE {
			return visitorFunc(f.findTypes)
		}
	case *ast.TypeSpec:
		f.n[n.Name.Name] = n
		return visitorFunc(f.findTypes)
	}

	return nil
}