package audit

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

type Recorder struct {
	db       *gorm.DB
	log      *slog.Logger
	ipPepper []byte
}

func New(db *gorm.DB, log *slog.Logger, ipPepper []byte) *Recorder {
	return &Recorder{db: db, log: log, ipPepper: ipPepper}
}

// Audit writes are part of the action's transaction when tx is non-nil, so an
// action can never commit without its audit row.
func (r *Recorder) Log(tx *gorm.DB, actor model.ActorType, actorID *uuid.UUID, verb, targetType string, targetID *uuid.UUID, meta model.JSONMap) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Create(&model.AuditLog{ActorType: actor, ActorID: actorID, Verb: verb, TargetType: targetType, TargetID: targetID, Meta: meta}).Error
}

func (r *Recorder) Security(ctx context.Context, kind model.SecurityEventKind, userID *uuid.UUID, ip string, meta model.JSONMap) {
	ev := model.SecurityEvent{Kind: kind, UserID: userID, Meta: meta}
	if ip != "" {
		ev.IPHash = secure.HMAC(r.ipPepper, ip)
	}
	if err := r.db.WithContext(ctx).Create(&ev).Error; err != nil {
		r.log.Error("security event write failed", "kind", string(kind), "err", err.Error())
	}
}

func Ptr(id uuid.UUID) *uuid.UUID { return &id }
