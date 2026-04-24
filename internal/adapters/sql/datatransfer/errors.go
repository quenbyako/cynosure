package datatransfer

import (
	"errors"
)

var ErrMaxContextOverflow = errors.New("max context messages overflowed int32")
