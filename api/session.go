package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/unweave/unweave-v2/model"
	"github.com/unweave/unweave-v2/session/runtime"
)

// ---------------------------------------------------------------------------------------
// SessionCreate POST /session
// ---------------------------------------------------------------------------------------

// swagger:parameters sessionCreate
type sessionCreateRequest struct {
	// in: body
	Body SessionCreateRequest
}

type SessionCreateRequest struct {
	Runtime model.RuntimeProvider `json:"runtime"`
}

func (s *SessionCreateRequest) Bind(r *http.Request) error {
	if s.Runtime == "" {
		return errors.New("field `runtime` is required")
	}
	if s.Runtime != model.LambdaLabsProvider && s.Runtime != model.UnweaveProvider {
		return fmt.Errorf("invalid runtime provider: %s. Must be one of `%s` or `%s`", s.Runtime, model.LambdaLabsProvider, model.UnweaveProvider)
	}
	return nil
}

// swagger:response sessionCreate
type sessionCreateResponse struct {
	// in: body
	Body SessionCreateResponse
}

type SessionCreateResponse struct {
	ID string `json:"id"`
}

// ---------------------------------------------------------------------------------------
// SessionGet GET /session/{id}
// ---------------------------------------------------------------------------------------

// swagger:response sessionGet
type sessionGetResponse struct {
	// in: body
	Body SessionGetResponse
}

type SessionGetResponse struct {
	ID     string         `json:"id"`
	Status runtime.Status `json:"runtimeStatus"`
}

// ---------------------------------------------------------------------------------------
// SessionConnect PUT /session/{id}/connect
// ---------------------------------------------------------------------------------------

// swagger:response sessionConnect
type sessionConnectResponse struct {
	// in: body
	Body SessionConnectResponse
}

type SessionConnectResponse struct {
	ID         string                `json:"id"`
	Status     runtime.Status        `json:"runtimeStatus"`
	Connection runtime.SSHConnection `json:"sshConnection"`
}
