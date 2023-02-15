package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/unweave/unweave/api/types"
	"github.com/unweave/unweave/db"
	"github.com/unweave/unweave/runtime"
	"github.com/unweave/unweave/tools"
	"github.com/unweave/unweave/tools/random"
	"golang.org/x/crypto/ssh"
)

func registerCredentials(ctx context.Context, rt *runtime.Runtime, key types.SSHKey) error {
	// Check if it exists with the provider and exit early if it does
	providerKeys, err := rt.ListSSHKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to list ssh keys from provider: %w", err)
	}
	for _, k := range providerKeys {
		if k.Name == key.Name {
			return nil
		}
	}
	if _, err = rt.AddSSHKey(ctx, key); err != nil {
		return fmt.Errorf("failed to add ssh key to provider: %w", err)
	}
	return nil
}

func fetchCredentials(ctx context.Context, accountID uuid.UUID, sshKeyName, sshPublicKey *string) (types.SSHKey, error) {
	if sshKeyName == nil && sshPublicKey == nil {
		return types.SSHKey{}, &types.Error{
			Code:    http.StatusBadRequest,
			Message: "Either Key name or Public Key must be provided",
		}
	}

	if sshKeyName != nil {
		params := db.SSHKeyGetByNameParams{Name: *sshKeyName, OwnerID: accountID}
		k, err := db.Q.SSHKeyGetByName(ctx, params)
		if err == nil {
			return types.SSHKey{
				Name:      k.Name,
				PublicKey: &k.PublicKey,
				CreatedAt: &k.CreatedAt,
			}, nil
		}
		if err != sql.ErrNoRows {
			return types.SSHKey{}, &types.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to get SSH key",
				Err:     fmt.Errorf("failed to get ssh key from db: %w", err),
			}
		}
	}

	// Not found by name, try public key
	if sshPublicKey != nil {
		pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(*sshPublicKey))
		if err != nil {
			return types.SSHKey{}, &types.Error{
				Code:    http.StatusBadRequest,
				Message: "Invalid SSH public key",
			}
		}

		pkStr := string(ssh.MarshalAuthorizedKey(pk))
		params := db.SSHKeyGetByPublicKeyParams{PublicKey: pkStr, OwnerID: accountID}
		k, err := db.Q.SSHKeyGetByPublicKey(ctx, params)
		if err == nil {
			return types.SSHKey{
				Name:      k.Name,
				PublicKey: &k.PublicKey,
				CreatedAt: &k.CreatedAt,
			}, nil
		}
		if err != sql.ErrNoRows {
			return types.SSHKey{}, &types.Error{
				Code:    http.StatusInternalServerError,
				Message: "Failed to get SSH key",
				Err:     fmt.Errorf("failed to get ssh key from db: %w", err),
			}
		}
	}

	// Public key wasn't provided	 and key wasn't found by name
	if sshPublicKey == nil {
		return types.SSHKey{}, &types.Error{
			Code:    http.StatusBadRequest,
			Message: "SSH key not found",
		}
	}
	if sshKeyName == nil || *sshKeyName == "" {
		sshKeyName = tools.Stringy("uw:" + random.GenerateRandomPhrase(4, "-"))
	}

	// Key doesn't exist in db, but the user provided a public key, so add it to the db
	if err := saveSSHKey(ctx, accountID, *sshKeyName, *sshPublicKey); err != nil {
		return types.SSHKey{}, &types.Error{
			Code:    http.StatusInternalServerError,
			Message: "Failed to save SSH key",
		}
	}
	return types.SSHKey{
		Name:      *sshKeyName,
		PublicKey: sshPublicKey,
	}, nil
}

type SessionService struct {
	srv *Service
}

func (s *SessionService) Create(ctx context.Context, projectID uuid.UUID, params types.SessionCreateParams) (*types.Session, error) {
	rt, err := s.srv.rti.Initialize(ctx, s.srv.cid, params.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	ctx = log.With().
		Stringer(types.RuntimeProviderKey, rt.GetProvider()).
		Logger().
		WithContext(ctx)

	sshKey, err := fetchCredentials(ctx, s.srv.cid, params.SSHKeyName, params.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to setup credentials: %w", err)
	}
	if err = registerCredentials(ctx, rt, sshKey); err != nil {
		return nil, fmt.Errorf("failed to register credentials: %w", err)
	}

	node, err := rt.InitNode(ctx, sshKey, params.NodeTypeID, params.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %w", err)
	}

	dbp := db.SessionCreateParams{
		NodeID:     node.ID,
		CreatedBy:  s.srv.cid,
		ProjectID:  projectID,
		Provider:   params.Provider.String(),
		Region:     node.Region,
		Name:       random.GenerateRandomPhrase(4, "-"),
		SshKeyName: sshKey.Name,
	}
	sessionID, err := db.Q.SessionCreate(ctx, dbp)
	if err != nil {
		return nil, fmt.Errorf("failed to create session in db: %w", err)
	}

	session := &types.Session{
		ID:         sessionID,
		SSHKey:     node.KeyPair,
		Status:     types.StatusInitializing,
		NodeTypeID: node.TypeID,
		Region:     node.Region,
		Provider:   node.Provider,
	}

	return session, nil
}

func (s *SessionService) Get(ctx context.Context, sessionID uuid.UUID) (*types.Session, error) {
	dbs, err := db.Q.MxSessionGet(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &types.Error{
				Code:    http.StatusNotFound,
				Message: "Session not found",
			}
		}
		return nil, fmt.Errorf("failed to get session from db: %w", err)
	}

	session := &types.Session{
		ID: sessionID,
		SSHKey: types.SSHKey{
			Name:      dbs.SshKeyName,
			PublicKey: &dbs.PublicKey,
			CreatedAt: &dbs.SshKeyCreatedAt,
		},
		Status:     types.SessionStatus(dbs.Status),
		CreatedAt:  &dbs.CreatedAt,
		NodeTypeID: dbs.NodeID,
		Region:     dbs.Region,
		Provider:   types.RuntimeProvider(dbs.Provider),
	}
	return session, nil
}

func (s *SessionService) List(ctx context.Context, projectID uuid.UUID, listTerminated bool) ([]types.Session, error) {
	params := db.SessionsGetParams{
		ProjectID: projectID,
		Limit:     100,
		Offset:    0,
	}
	sessions, err := db.Q.SessionsGet(ctx, params)
	if err != nil {
		err = fmt.Errorf("failed to get sessions from db: %w", err)
		return nil, err
	}

	var res []types.Session
	for _, s := range sessions {
		s := s
		if !listTerminated && s.Status == db.UnweaveSessionStatusTerminated {
			continue
		}
		sess := types.Session{
			ID: s.ID,
			SSHKey: types.SSHKey{
				// The generated go type for SshKeyName is a nullable string because
				// of the join, but it will never be null since session have a foreign
				// key constraint on ssh_key_id.
				Name: s.SshKeyName.String,
			},
			Status: types.DBSessionStatusToAPIStatus(s.Status),
		}
		res = append(res, sess)
	}
	return res, nil
}

func (s *SessionService) Watch(ctx context.Context, sessionID uuid.UUID) error {
	session, err := db.Q.SessionGet(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &types.Error{
				Code:    http.StatusNotFound,
				Message: "Session not found",
			}
		}
		return fmt.Errorf("failed to get session from db: %w", err)
	}

	rt, err := s.srv.rti.Initialize(ctx, s.srv.cid, types.RuntimeProvider(session.Provider))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	statusch, errch := rt.Watch(ctx, session.NodeID)

	log.Ctx(ctx).Info().Msgf("Starting to watch session %s", sessionID)

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case status := <-statusch:
				log.Ctx(ctx).
					Info().
					Str(SessionStatusCtxKey, string(status)).
					Msg("session status changed")

				params := db.SessionStatusUpdateParams{
					ID:     sessionID,
					Status: db.UnweaveSessionStatus(status),
				}
				if e := db.Q.SessionStatusUpdate(ctx, params); e != nil {
					log.Ctx(ctx).Error().Err(e).Msg("failed to update session status")
					return
				}
				if status == types.StatusTerminated {
					return
				}
			case e := <-errch:
				log.Ctx(ctx).Error().Err(e).Msg("failed to watch session")
				return
			}
		}
	}()

	return nil
}

func (s *SessionService) Terminate(ctx context.Context, sessionID uuid.UUID) error {
	sess, err := db.Q.SessionGet(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &types.Error{
				Code:       http.StatusNotFound,
				Message:    "Session not found",
				Suggestion: "Make sure the session id is valid",
			}
		}
		return fmt.Errorf("failed to fetch session from db %q: %w", sessionID, err)
	}

	provider := types.RuntimeProvider(sess.Provider)
	rt, err := s.srv.rti.Initialize(ctx, s.srv.cid, provider)
	if err != nil {
		return fmt.Errorf("failed to create runtime %q: %w", sess.Provider, err)
	}

	ctx = log.With().
		Stringer(types.RuntimeProviderKey, rt.GetProvider()).
		Logger().
		WithContext(ctx)

	if err = rt.TerminateNode(ctx, sess.NodeID); err != nil {
		return fmt.Errorf("failed to terminate node: %w", err)
	}
	params := db.SessionStatusUpdateParams{
		ID:     sessionID,
		Status: db.UnweaveSessionStatusTerminated,
	}
	if err = db.Q.SessionStatusUpdate(ctx, params); err != nil {
		log.Ctx(ctx).
			Error().
			Err(err).
			Msgf("Failed to set session %q as terminated", sessionID)
	}
	return nil
}
