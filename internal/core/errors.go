package core

import (
	"errors"
	"fmt"

	"github.com/mathieu-sabatier/twin-model/internal/dto"
)

// Sentinel errors returned by Service. Each transport maps them to its own
// vocabulary (HTTP status, MCP tool error).
var (
	ErrNotFound = errors.New("not found")        // unknown/expired draft, missing file/ref, unknown spec/type
	ErrParse    = errors.New("parse error")      // structural YAML parse failure (HTTP 422)
	ErrInvalid  = errors.New("invalid argument") // missing/blank required argument (HTTP 400)
	ErrReadTree = errors.New("read tree")        // git host read failure (HTTP 502)
	ErrInternal = errors.New("internal error")   // catalog load/deps failure (HTTP 500)
)

// ValidationError reports that a draft cannot be proposed because one or more
// model files are lint-red or unparseable. Mirrors the API's 409 payload.
type ValidationError struct{ Blocking []dto.Diagnostic }

func (e *ValidationError) Error() string {
	return fmt.Sprintf("model has %d blocking diagnostic(s)", len(e.Blocking))
}
