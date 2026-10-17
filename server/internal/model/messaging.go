package model

import (
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	SoftBase
	Kind               ConversationKind `gorm:"size:12;not null;index"`
	Title              *string          `gorm:"size:60"`
	Topic              *string          `gorm:"size:200"`
	BowlID             *uuid.UUID       `gorm:"type:uuid;index"`
	MatchID            *uuid.UUID       `gorm:"type:uuid;uniqueIndex"`
	CrewID             *uuid.UUID       `gorm:"type:uuid;uniqueIndex"`
	StartsAt           *time.Time       `gorm:"index"`
	EndsAt             *time.Time       `gorm:"index"`
	ClosedAt           *time.Time
	SlowModeSec        int16 `gorm:"not null;default:0"`
	DisappearAfterSec  *int
	MemberCap          int16      `gorm:"not null;default:2"`
	InviteCode         *string    `gorm:"size:16;uniqueIndex"`
	LastMessageAt      *time.Time `gorm:"index"`
	CreatedByPersonaID *uuid.UUID `gorm:"type:uuid"`
}

type ConversationMember struct {
	ConversationID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	PersonaID              uuid.UUID  `gorm:"type:uuid;primaryKey;index"`
	Role                   ConvRole   `gorm:"size:8;not null;default:member"`
	LastReadMessageID      *uuid.UUID `gorm:"type:uuid"`
	LastDeliveredMessageID *uuid.UUID `gorm:"type:uuid"`
	MutedUntil             *time.Time
	RevealLevel            RevealLevel `gorm:"size:12;not null;default:none"`
	JoinedAt               time.Time   `gorm:"not null"`
	LeftAt                 *time.Time  `gorm:"index"`
	LastSentAt             *time.Time
}

type DMRequest struct {
	Base
	FromPersonaID  uuid.UUID     `gorm:"type:uuid;not null;index"`
	ToPersonaID    uuid.UUID     `gorm:"type:uuid;not null;index"`
	IntroMasked    string        `gorm:"size:200"`
	ContextType    DMContext     `gorm:"size:10;not null"`
	ContextID      *uuid.UUID    `gorm:"type:uuid"`
	Status         RequestStatus `gorm:"size:10;not null;index;default:pending"`
	ConversationID *uuid.UUID    `gorm:"type:uuid"`
}

type Message struct {
	ID              uuid.UUID     `gorm:"type:uuid;primaryKey"`
	ConversationID  uuid.UUID     `gorm:"type:uuid;not null;index:idx_msg_conv_id,priority:1"`
	SenderPersonaID *uuid.UUID    `gorm:"type:uuid;index"`
	Kind            MessageKind   `gorm:"size:12;not null"`
	BodyMasked      string        `gorm:"type:text"`
	Media           MediaList     `gorm:"type:text"`
	ReplyToID       *uuid.UUID    `gorm:"type:uuid"`
	PIIFlags        int16         `gorm:"not null;default:0"`
	Status          ContentStatus `gorm:"size:20;not null;default:visible"`
	CreatedAt       time.Time     `gorm:"not null;index:idx_msg_conv_id,priority:2"`
	EditedAt        *time.Time
	ExpiresAt       *time.Time `gorm:"index"`
	// View-once photos: each recipient may open it a single time; the file is deleted after.
	ViewOnce bool `gorm:"not null;default:false"`
}

// MessageView records that a member opened a view-once photo.
type MessageView struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ViewedAt  time.Time `gorm:"not null"`
}

type RevealEvent struct {
	Base
	ConversationID uuid.UUID    `gorm:"type:uuid;not null;index"`
	PersonaID      uuid.UUID    `gorm:"type:uuid;not null"`
	Level          RevealLevel  `gorm:"size:12;not null"`
	Action         RevealAction `gorm:"size:8;not null"`
}

// What a user chose to share once a reveal level is mutually granted. Encrypted: it is real identity.
type RevealProfile struct {
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	FirstNameEnc []byte
	PhotoKey     *string `gorm:"size:200"`
	ContactEnc   []byte
	UpdatedAt    time.Time
}

func init() {
	register(&Conversation{}, &ConversationMember{}, &DMRequest{}, &Message{}, &RevealEvent{}, &RevealProfile{}, &MessageView{})
}
