package cynosure

type MissingParamError string

func (e MissingParamError) Error() string {
	return "missing " + string(e)
}
