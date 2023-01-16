package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

// Error
//
// Errors returned by the API should be as descriptive as possible and directly renderable
// to the consumer (CLI, web-app etc). Here are some examples:
//
// Provider errors
// ---------------
// Short:
//
//	LambdaLabs API error: Invalid Public Key
//
// Verbose:
//
//	LambdaLabs API error:
//		code: 400
//		message: Invalid Public Key
//	 	endpoint: POST /session
//
// Unweave errors
// --------------
// Short:
//
//	Unweave API error: Project not found
//
// Verbose:
//
//	Unweave API error:
//		code: 404
//		message: Project not found
//	 	endpoint: POST /session
//
// It should be possible to automatically generate the short and verbose versions of the
// error message from the same struct. The error message should not expose in inner workings
// of the API.
type Error struct {
	Code       int             `json:"code"`
	Message    string          `json:"message"`
	Suggestion string          `json:"suggestion"`
	Provider   RuntimeProvider `json:"provider"`
	Err        error           `json:"error"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

type UwError interface {
	Short() string
	Verbose() string
}

type HTTPError struct {
	Code       int             `json:"code"`
	Message    string          `json:"message"`
	Suggestion string          `json:"suggestion"`
	Provider   RuntimeProvider `json:"provider"`
	Err        error           `json:"-"`
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *HTTPError) Render(w http.ResponseWriter, r *http.Request) error {
	// Depending on whether it is Unweave's fault or the user's fault, log the error
	// appropriately.
	if e.Code == http.StatusInternalServerError {
		log.Ctx(r.Context()).Error().Err(e).Stack().Msg(e.Message)
	} else {
		log.Ctx(r.Context()).Warn().Err(e).Stack().Msg(e.Message)
	}
	render.Status(r, e.Code)
	return nil
}

func ErrHTTPError(err error, fallbackMessage string) render.Renderer {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return &HTTPError{
			Code:       e.Code,
			Message:    e.Message,
			Provider:   e.Provider,
			Suggestion: e.Suggestion,
			Err:        e.Err,
		}
	}
	return ErrInternalServer(err, fallbackMessage)
}

func ErrInternalServer(err error, msg string) render.Renderer {
	m := "Internal server error"
	if msg != "" {
		m = msg
	}
	return &HTTPError{
		Code:    http.StatusInternalServerError,
		Message: m,
		Err:     err,
	}
}
