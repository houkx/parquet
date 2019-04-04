package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"strings"
	"text/template"
)

var (
	pkg    = flag.String("package", "main", "package of the generated code")
	max    = flag.Int("max", 3, "the bit width at which to stop")
	outPth = flag.String("output", "bitpack.go", "name of the file that is produced, defaults to parquet.go")
)

func main() {
	flag.Parse()
	pb := bitback{Package: *pkg, Max: *max}
	tmpl := template.New("output").Funcs(funcs)
	var err error
	tmpl, err = tmpl.Parse(tpl)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range []string{
		bytesTpl,
		intsTpl,
	} {
		var err error
		tmpl, err = tmpl.Parse(t)
		if err != nil {
			log.Fatal(err)
		}
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, pb)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(buf.Bytes()))

	// gocode, err := format.Source(buf.Bytes())
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// f, err := os.Create(*outPth)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// _, err = f.Write(gocode)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// f.Close()
}

type bitback struct {
	Package string
	Max     int
}

var (
	funcs = template.FuncMap{
		"byte": func(width, i, j int) string {
			index := ((i - 1) * 8) + j
			x := ((index / width) * width) - (((index / 8) % 8) * 8)
			dir := "<<"
			if x < 0 {
				dir = ">>"
				x = x * -1
			}
			shift := fmt.Sprintf("%s %d", dir, x)
			return fmt.Sprintf("byte((vals[%d]&%d)%s)", index/width, 1<<uint(index%width), shift)
		},
		"int64": func(width, i int) string {
			mask := ((1 << uint(width)) - 1) << uint(i*width)
			shift := (i * width) % 8
			index := (i / width) * width
			var parts []string
			if shift < (i * width) { //not correct
				parts = []string{
					fmt.Sprintf("(int64(vals[%d] & %d) >> %d)", index, mask, shift), //not correct
					fmt.Sprintf("(int64(vals[%d] & %d) >> %d)", index, mask, shift), //not correct
				}
			} else {
				parts = []string{
					fmt.Sprintf("(int64(vals[%d] & %d) >> %d)", index, mask, shift),
				}
			}

			return fmt.Sprintf("%s,", strings.Join(parts, " | "))
		},
		"N": func(start, end int) (stream chan int) {
			stream = make(chan int)
			go func() {
				for i := start; i <= end; i++ {
					stream <- i
				}
				close(stream)
			}()
			return
		},
	}

	// 	tpl = `package {{.Package}}

	// // This code is generated by github.com/parsyl/parquet.

	// {{range $i := N 1 {{.Max}}}}
	// func pack{{$i}}(vals []int64) []byte { {{template "bytes" .}}
	// }
	// {{end}}
	// `

	tpl = `package {{.Package}}

// This code is generated by github.com/parsyl/parquet.

{{range $i := N 1 .Max }}
func unpack{{$i}}(vals []byte) []int64 { {{template "ints" .}}
}
{{end}}
`

	intsTpl = `{{define "ints"}}{{$width := .}}
return []int64{
{{range $i := N 0 7}} {{int64 $width $i}}
{{end}} }{{end}}`

	bytesTpl = `{{define "bytes"}}{{$width := .}}
return []byte{
{{range $i := N 1 .}} ({{range $j := N 0 6}}{{byte $width $i $j}} |
  {{end}}{{byte $width $i 7}}),
{{end}} }{{end}}`
)