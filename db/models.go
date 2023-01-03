// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0

package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type UnweaveProject struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OwnerID uuid.UUID `json:"ownerID"`
}

type UnweaveSession struct {
	ID        uuid.UUID    `json:"id"`
	NodeID    string       `json:"nodeID"`
	CreatedBy uuid.UUID    `json:"createdBy"`
	CreatedAt time.Time    `json:"createdAt"`
	ReadyAt   sql.NullTime `json:"readyAt"`
	ExitedAt  sql.NullTime `json:"exitedAt"`
	ProjectID uuid.UUID    `json:"projectID"`
	Runtime   string       `json:"runtime"`
}

type UnweaveSshKey struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	OwnerID   uuid.UUID `json:"ownerID"`
	CreatedAt time.Time `json:"createdAt"`
	PublicKey string    `json:"publicKey"`
}

type UnweaveUser struct {
	ID uuid.UUID `json:"id"`
}
