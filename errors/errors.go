package errors

// Error is an immutable and const error
type Error string

func (e Error) Error() string { return string(e) }
