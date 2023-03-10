package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
	"github.com/unweave/unweave/api/types"
	"github.com/unweave/unweave/db"
	"github.com/unweave/unweave/runtime"
)

// Builder

// BuildsCreate expects a request body containing both the build context and the json
// params for the build.
//
//		eg. curl -X POST \
//				 -H 'Authorization: Bearer <token>' \
//	 		 	 -H 'Content-Type: multipart/form-data' \
//	 		 	 -F context=@context.zip \
//	 		 	 -F 'params={"builder": "docker"}'
//				 https://<api-host>/builds
func BuildsCreate(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing BuildsCreate request")

		ibp := &types.BuildsCreateParams{}
		if err := ibp.Bind(r); err != nil {
			err = fmt.Errorf("failed to read body: %w", err)
			render.Render(w, r.WithContext(ctx), ErrHTTPBadRequest(err, "Invalid request body"))
			return
		}

		accountID := GetAccountIDFromContext(ctx)
		projectID := GetProjectIDFromContext(ctx)

		srv := NewCtxService(rti, accountID)

		buildID, err := srv.Builder.Build(ctx, projectID, ibp)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to build image"))
			return
		}

		res := &types.BuildsCreateResponse{BuildID: buildID}
		render.JSON(w, r, res)
	}
}

// BuildsGet returns the details of a build. If the query param `logs` is set to
// true, the logs of the build will be returned as well.
func BuildsGet(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing BuildsGet request")

		buildID := chi.URLParam(r, "buildID")
		getLogs := r.URL.Query().Get("logs") == "true"

		accountID := GetAccountIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		// get build from db
		build, err := db.Q.BuildGet(ctx, buildID)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to get build"))
			return
		}

		res := &types.BuildsGetResponse{
			BuildID: buildID,
			Status:  string(build.Status),
			Logs:    nil,
		}

		if getLogs {
			logs, err := srv.Builder.GetLogs(ctx, buildID)
			if err != nil {
				render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to get build logs"))
				return
			}
			res.Logs = &logs
		}
		render.JSON(w, r, res)
	}
}

// Provider

// NodeTypesList returns a list of node types available for the user. If the query param
// `available` is set to true, only node types that are currently available to be
// scheduled will be returned.
func NodeTypesList(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		provider := types.RuntimeProvider(chi.URLParam(r, "provider"))
		log.Ctx(ctx).Info().Msgf("Executing NodeTypesList request for provider %s", provider)

		filterAvailable := r.URL.Query().Get("available") == "true"

		accountID := GetAccountIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		nodeTypes, err := srv.Provider.ListNodeTypes(ctx, provider, filterAvailable)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to list node types"))
			return
		}

		res := &types.NodeTypesListResponse{NodeTypes: nodeTypes}
		render.JSON(w, r, res)
	}
}

// Sessions

func SessionsCreate(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing SessionsCreate request")

		scr := types.SessionCreateParams{}
		if err := render.Bind(r, &scr); err != nil {
			err = fmt.Errorf("failed to read body: %w", err)
			render.Render(w, r.WithContext(ctx), ErrHTTPBadRequest(err, "Invalid request body"))
			return
		}

		accountID := GetAccountIDFromContext(ctx)
		projectID := GetProjectIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		session, err := srv.Session.Create(ctx, projectID, scr)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to create session"))
			return
		}

		go func() {
			c := context.Background()
			c = log.With().
				Stringer(AccountIDCtxKey, accountID).
				Str(ProjectIDCtxKey, projectID).
				Str(SessionIDCtxKey, session.ID).
				Logger().WithContext(c)

			if e := srv.Session.Watch(c, session.ID); e != nil {
				log.Ctx(ctx).Error().Err(e).Msgf("Failed to watch session")
			}
		}()

		render.JSON(w, r, session)
	}
}

func SessionsGet(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing SessionsGet request")

		accountID := GetAccountIDFromContext(ctx)
		sessionID := GetSessionIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		session, err := srv.Session.Get(ctx, sessionID)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to get session"))
			return
		}

		render.JSON(w, r, types.SessionGetResponse{Session: *session})
	}
}

func SessionsList(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountIDFromContext(ctx)
		projectID := GetProjectIDFromContext(ctx)
		listTerminated := r.URL.Query().Get("terminated") == "true"

		log.Ctx(ctx).Info().Msgf("Executing SessionsList request")

		srv := NewCtxService(rti, accountID)
		sessions, err := srv.Session.List(ctx, projectID, listTerminated)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to list sessions"))
			return
		}
		render.JSON(w, r, types.SessionsListResponse{Sessions: sessions})
	}
}

func SessionsTerminate(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		accountID := GetAccountIDFromContext(ctx)

		log.Ctx(ctx).
			Info().
			Msgf("Executing SessionsTerminate request")

		sessionID := GetSessionIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		if err := srv.Session.Terminate(ctx, sessionID); err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to terminate session"))
			return
		}
		render.Status(r, http.StatusOK)
	}
}

// SSH Keys

// SSHKeyAdd adds an SSH key to the user's account.
//
// This does not add the key to the user's configured providers. That is done lazily
// when the user first tries to use the key.
func SSHKeyAdd(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing SSHKeyAdd request")

		params := types.SSHKeyAddParams{}
		if err := render.Bind(r, &params); err != nil {
			err = fmt.Errorf("failed to read body: %w", err)
			render.Render(w, r.WithContext(ctx), ErrHTTPBadRequest(err, "Invalid request body"))
			return
		}

		accountID := GetAccountIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		if err := srv.SSHKey.Add(ctx, params); err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to add SSH key"))
			return
		}
		render.JSON(w, r, &types.SSHKeyAddResponse{Success: true})
	}
}

func SSHKeyGenerate(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing SSHKeyCreate request")

		params := types.SSHKeyGenerateParams{}
		if err := render.Bind(r, &params); err != nil {
			err = fmt.Errorf("failed to read body: %w", err)
			render.Render(w, r.WithContext(ctx), ErrHTTPBadRequest(err, "Invalid request body"))
			return
		}

		accountID := GetAccountIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		name, prv, pub, err := srv.SSHKey.Generate(ctx, params)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to generate SSH key"))
			return
		}

		res := types.SSHKeyGenerateResponse{
			Name:       name,
			PublicKey:  pub,
			PrivateKey: prv,
		}
		render.JSON(w, r, &res)
	}
}

func SSHKeyList(rti runtime.Initializer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log.Ctx(ctx).Info().Msgf("Executing SSHKeyList request")

		accountID := GetAccountIDFromContext(ctx)
		srv := NewCtxService(rti, accountID)

		keys, err := srv.SSHKey.List(ctx)
		if err != nil {
			render.Render(w, r.WithContext(ctx), ErrHTTPError(err, "Failed to list SSH keys"))
			return
		}

		res := types.SSHKeyListResponse{Keys: keys}
		render.JSON(w, r, res)
	}
}
