package messages

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"

	"github.com/nikolalohinski/gonja"
	"github.com/nikolalohinski/gonja/config"
	"github.com/nikolalohinski/gonja/nodes"
	"github.com/nikolalohinski/gonja/parser"
	"github.com/slongfield/pyfmt"
)

// TODO: it's not a component for sure

func formatContent(
	content string,
	properties map[string]any,
	formatType FormatType,
) (string, error) {
	switch formatType {
	case FString:
		return pyfmt.Fmt(content, properties)
	case GoTemplate:
		parsedTmpl, err := template.New("template").
			Option("missingkey=error").
			Parse(content)
		if err != nil {
			return "", err
		}

		builder := new(strings.Builder)

		err = parsedTmpl.Execute(builder, properties)
		if err != nil {
			return "", err
		}

		return builder.String(), nil
	case Jinja2:
		env, err := getJinjaEnv()
		if err != nil {
			return "", err
		}

		tpl, err := env.FromString(content)
		if err != nil {
			return "", err
		}

		out, err := tpl.Execute(properties)
		if err != nil {
			return "", err
		}

		return out, nil
	default:
		return "", fmt.Errorf("unknown format type: %v", formatType)
	}
}

// custom jinja env
var (
	jinjaEnvOnce sync.Once
	jinjaEnv     *gonja.Environment
	envInitErr   error
)

const (
	jinjaInclude = "include"
	jinjaExtends = "extends"
	jinjaImport  = "import"
	jinjaFrom    = "from"
)

func staticErr(err error) parser.StatementParser {
	return func(parser, args *parser.Parser) (nodes.Statement, error) {
		return nil, err
	}
}

func getJinjaEnv() (*gonja.Environment, error) {
	jinjaEnvOnce.Do(func() {
		jinjaEnv = gonja.NewEnvironment(config.DefaultConfig, gonja.DefaultLoader)
		formatInitError := "init jinja env fail: %w"

		var err error
		if jinjaEnv.Statements.Exists(jinjaInclude) {
			err = jinjaEnv.Statements.Replace(jinjaInclude,
				staticErr(errors.New("keyword[include] has been disabled")),
			)
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}

		if jinjaEnv.Statements.Exists(jinjaExtends) {
			err = jinjaEnv.Statements.Replace(jinjaExtends,
				staticErr(errors.New("keyword[extends] has been disabled")),
			)
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}

		if jinjaEnv.Statements.Exists(jinjaFrom) {
			err = jinjaEnv.Statements.Replace(jinjaFrom,
				staticErr(errors.New("keyword[from] has been disabled")),
			)
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}

		if jinjaEnv.Statements.Exists(jinjaImport) {
			err = jinjaEnv.Statements.Replace(jinjaImport,
				staticErr(errors.New("keyword[import] has been disabled")),
			)
			if err != nil {
				envInitErr = fmt.Errorf(formatInitError, err)
				return
			}
		}
	})

	return jinjaEnv, envInitErr
}
