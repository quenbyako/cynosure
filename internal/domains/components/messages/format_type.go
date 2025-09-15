package messages

// FormatType used by MessageTemplate.Format
type FormatType uint8

const (
	// FString Supported by pyfmt(github.com/slongfield/pyfmt), which is an implementation of https://peps.python.org/pep-3101/.
	FString FormatType = 0
	// GoTemplate https://pkg.go.dev/text/template.
	GoTemplate FormatType = 1
	// Jinja2 Supported by gonja(github.com/nikolalohinski/gonja), which is a implementation of https://jinja.palletsprojects.com/en/3.1.x/templates/.
	Jinja2 FormatType = 2
)
