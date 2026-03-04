//go:build ignore

package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"gopkg.in/yaml.v3"
)

//go:embed *.tmpl
var templates embed.FS

func main() {
	var data any

	prefix := flag.String("cut-prefix", "", "prefix to cut semconv entities")
	flag.Parse()

	yaml.NewDecoder(os.Stdin).Decode(&data)

	templates := template.Must(template.New("").Funcs(template.FuncMap{
		// ToCamel converts a string to `CamelCase`
		"ToCamel": strcase.ToCamel,
		// ToLowerCamel converts a string to `lowerCamelCase`
		"ToLowerCamel": strcase.ToLowerCamel,
		// CutPrefix cuts a prefix from a string, e.g. "yourapp.some.name" -> "some.name"
		"TrimPrefix": func(s string) string {
			return strings.TrimPrefix(s, *prefix)
		},
		// ToType converts a semconv types into go types
		"ToType": func(typ, attrName string) string {
			switch attrName {
			case string(semconv.ErrorTypeKey):
				return "error"
			case string("process.environment_variable"): // tricky attr, defined in semconv, but has only function.
				return "map[string]string"
			case string("cynosure.listen.addr"):
				return "net.Addr"
			default:
				return typ
			}
		},
		"IsExploding": func(typ, attrName string) bool {
			_, ok := map[string]struct{}{
				"process.environment_variable": {},
			}[attrName]
			return ok
		},
		"Convert": func(typ, attrName, goKey, varname string) (value string) {
			switch attrName {
			case string(semconv.ErrorTypeKey),
				string("cynosure.listen.addr"):
				return fmt.Sprintf("%s.String(fmt.Sprintf(\"%%v\", %v))", goKey, varname)

			case string("process.environment_variable"):
				return fmt.Sprintf("asEnvs(%v)", varname)

			default:
				return varname
			}

		},
	}).ParseFS(templates, "*.tmpl"))

	if err := templates.ExecuteTemplate(os.Stdout, "logs.tmpl", data); err != nil {
		panic(err)
	}
}
