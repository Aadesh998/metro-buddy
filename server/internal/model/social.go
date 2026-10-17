package model

import (
	"time"

	"github.com/google/uuid"
)

// Follow is one-way and needs no consent: it only adds the followee's posts to the follower's feed.
type Follow struct {
	FollowerPersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	FolloweePersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	CreatedAt         time.Time `gorm:"not null"`
}

// Connection is mutual and consent-gated. One row per pair of personas, keyed by PairKey.
// Once accepted, either side can open a chat without a message request.
type Connection struct {
	Base
	FromPersonaID uuid.UUID        `gorm:"type:uuid;not null;index"`
	ToPersonaID   uuid.UUID        `gorm:"type:uuid;not null;index"`
	PairKey       string           `gorm:"size:73;not null;uniqueIndex"`
	NoteMasked    string           `gorm:"size:140"`
	Status        ConnectionStatus `gorm:"size:10;not null;index;default:pending"`
	RespondedAt   *time.Time
}

func PairKey(a, b uuid.UUID) string {
	if a.String() > b.String() {
		a, b = b, a
	}
	return a.String() + "|" + b.String()
}

func init() {
	register(&Follow{}, &Connection{})
}
