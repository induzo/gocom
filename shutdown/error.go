package shutdown

import (
	"fmt"
	"strings"
)

type shutdownError map[string]error

func newShutdownError() shutdownError {
	return shutdownError(make(map[string]error))
}

func (em shutdownError) Error() string {
	errStr := make([]string, len(em))
	i := 0

	for n, e := range em {
		errStr[i] = fmt.Sprintf("%s err: %s", n, e)
		i++
	}

	return strings.Join(errStr, ", ")
}
