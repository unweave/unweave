// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0

package db

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	MxSessionGet(ctx context.Context, id uuid.UUID) (MxSessionGetRow, error)
	ProjectGet(ctx context.Context, id uuid.UUID) (UnweaveProject, error)
	SSHKeyAdd(ctx context.Context, arg SSHKeyAddParams) error
	SSHKeyGetByName(ctx context.Context, arg SSHKeyGetByNameParams) (UnweaveSshKey, error)
	SSHKeyGetByPublicKey(ctx context.Context, arg SSHKeyGetByPublicKeyParams) (UnweaveSshKey, error)
	SSHKeysGet(ctx context.Context, ownerID uuid.UUID) ([]UnweaveSshKey, error)
	SessionCreate(ctx context.Context, arg SessionCreateParams) (uuid.UUID, error)
	SessionGet(ctx context.Context, id uuid.UUID) (UnweaveSession, error)
	SessionStatusUpdate(ctx context.Context, arg SessionStatusUpdateParams) error
	SessionsGet(ctx context.Context, arg SessionsGetParams) ([]SessionsGetRow, error)
}

var _ Querier = (*Queries)(nil)
