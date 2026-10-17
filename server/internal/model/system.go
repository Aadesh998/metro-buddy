package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	Base
	PersonaID uuid.UUID        `gorm:"type:uuid;not null;index"`
	Kind      NotificationKind `gorm:"size:16;not null"`
	Payload   JSONMap          `gorm:"serializer:json;type:text"`
	ReadAt    *time.Time
}

type AdminUser struct {
	Base
	Email         string    `gorm:"size:120;not null;uniqueIndex"`
	Role          AdminRole `gorm:"size:20;not null"`
	PasswordHash  string    `gorm:"size:200;not null"`
	TOTPSecretEnc []byte
	TOTPConfirmed bool `gorm:"not null;default:false"`
	Active        bool `gorm:"not null;default:true"`
	FailedLogins  int  `gorm:"not null;default:0"`
	LockedUntil   *time.Time
}

type AuditLog struct {
	Base
	ActorType  ActorType  `gorm:"size:8;not null"`
	ActorID    *uuid.UUID `gorm:"type:uuid;index"`
	Verb       string     `gorm:"size:48;not null;index"`
	TargetType string     `gorm:"size:32"`
	TargetID   *uuid.UUID `gorm:"type:uuid;index"`
	Meta       JSONMap    `gorm:"serializer:json;type:text"`
}

type SecurityEvent struct {
	Base
	Kind   SecurityEventKind `gorm:"size:20;not null;index"`
	UserID *uuid.UUID        `gorm:"type:uuid;index"`
	IPHash []byte
	Meta   JSONMap `gorm:"serializer:json;type:text"`
}

type AIUsage struct {
	Base
	Purpose       AIPurpose  `gorm:"size:20;not null;index"`
	Model         string     `gorm:"size:48;not null"`
	InputTokens   int        `gorm:"not null"`
	OutputTokens  int        `gorm:"not null"`
	CostMicrosUSD int64      `gorm:"not null"`
	UserID        *uuid.UUID `gorm:"type:uuid;index"`
}

// Product analytics. Deliberately has no user or persona column: only a hashed, rotating session id.
type AnalyticsEvent struct {
	Base
	Name        string  `gorm:"size:48;not null;index"`
	SessionHash string  `gorm:"size:64;not null;index"`
	Platform    string  `gorm:"size:10"`
	AppVersion  string  `gorm:"size:16"`
	Props       JSONMap `gorm:"serializer:json;type:text"`
}

func init() {
	register(&Notification{}, &AdminUser{}, &AuditLog{}, &SecurityEvent{}, &AIUsage{}, &AnalyticsEvent{})
}
