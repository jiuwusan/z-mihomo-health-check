package domain

type InstanceError struct {
	Code    int
	Message string
}

func (e *InstanceError) Error() string               { return e.Message }
func InstanceFailure(code int, message string) error { return &InstanceError{code, message} }
