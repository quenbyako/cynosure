package messages

import (
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
		res, err := pyfmt.Fmt(content, properties)
		if err != nil {
			return "", fmt.Errorf("pyfmt: %w", err)
		}

		return res, nil
	case GoTemplate:
		return formatGoTemplate(content, properties)
	case Jinja2:
		return formatJinja2(content, properties)
	default:
		return "", ErrInternalValidation("unknown format type: %v", formatType)
	}
}

func formatGoTemplate(content string, properties map[string]any) (string, error) {
	parsedTmpl, err := template.New("template").
		Option("missingkey=error").
		Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse go template: %w", err)
	}

	builder := new(strings.Builder)

	err = parsedTmpl.Execute(builder, properties)
	if err != nil {
		return "", fmt.Errorf("execute go template: %w", err)
	}

	return builder.String(), nil
}

func formatJinja2(content string, properties map[string]any) (string, error) {
	env, err := getJinjaEnv()
	if err != nil {
		return "", err
	}

	tpl, err := env.FromString(content)
	if err != nil {
		return "", fmt.Errorf("parse jinja2 template: %w", err)
	}

	out, err := tpl.Execute(properties)
	if err != nil {
		return "", fmt.Errorf("execute jinja2 template: %w", err)
	}

	return out, nil
}

// custom jinja env
var (
	jinjaEnvOnce sync.Once
	jinjaEnv     *gonja.Environment
	errEnvInit   error
)

const (
	jinjaInclude = "include"
	jinjaExtends = "extends"
	jinjaImport  = "import"
	jinjaFrom    = "from"
)

func staticErr(err error) parser.StatementParser {
	return func(_, _ *parser.Parser) (nodes.Statement, error) {
		return nil, err
	}
}

func getJinjaEnv() (*gonja.Environment, error) {
	jinjaEnvOnce.Do(func() {
		jinjaEnv = gonja.NewEnvironment(config.DefaultConfig, gonja.DefaultLoader)
		disableJinjaKeywords(jinjaEnv)
	})

	return jinjaEnv, errEnvInit
}

func disableJinjaKeywords(env *gonja.Environment) {
	keywords := []string{jinjaInclude, jinjaExtends, jinjaFrom, jinjaImport}
	for _, keyword := range keywords {
		if env.Statements.Exists(keyword) {
			err := env.Statements.Replace(keyword,
				staticErr(ErrInternalValidation("keyword[%s] has been disabled", keyword)),
			)
			if err != nil {
				errEnvInit = fmt.Errorf("disable jinja keyword %s: %w", keyword, err)
				return
			}
		}
	}
}
