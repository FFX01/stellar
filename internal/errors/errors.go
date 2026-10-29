package errors

import "stellar/internal/lexer"
import "fmt"

type Error struct {
	Position lexer.Position
	Message  string
}

func (e *Error) String() string {
	return fmt.Sprintf("%s: %s", e.Position.String(), e.Message)
}
