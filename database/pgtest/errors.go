package pgtest

type ConnPoolNotFoundError struct{}

func (e *ConnPoolNotFoundError) Error() string {
	return "connpool is nil"
}
