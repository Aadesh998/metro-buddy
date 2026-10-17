package model

import (
	"time"

	"github.com/google/uuid"
)

type PIIVault struct {
	Base
	ContentType ContentType `gorm:"size:16;not null"`
	ContentID   uuid.UUID   `gorm:"type:uuid;not null;index"`
	RawEnc      []byte      `gorm:"not null"`
	KeyVersion  int16       `gorm:"not null"`
	ExpiresAt   time.Time   `gorm:"not null;index"`
}

type PIIDetection struct {
	Base
	ContentType ContentType `gorm:"size:16;not null"`
	ContentID   uuid.UUID   `gorm:"type:uuid;not null;index"`
	Kind        PIIKind     `gorm:"size:10;not null"`
	Source      Detector    `gorm:"size:6;not null"`
}

type SOSEvent struct {
	Base
	UserID        uuid.UUID  `gorm:"type:uuid;not null;index"`
	TripID        *uuid.UUID `gorm:"type:uuid"`
	LastStationID *uuid.UUID `gorm:"type:uuid"`
	Status        SOSStatus  `gorm:"size:20;not null;index;default:open"`
	ResolvedBy    *uuid.UUID `gorm:"type:uuid"`
}

type Report struct {
	Base
	ReporterPersonaID uuid.UUID   `gorm:"type:uuid;not null;index"`
	TargetType        ContentType `gorm:"size:16;not null;index:idx_report_target"`
	TargetID          uuid.UUID   `gorm:"type:uuid;not null;index:idx_report_target"`
	Reason            string      `gorm:"size:16;not null"`
	DetailsMasked     string      `gorm:"size:500"`
	CaseID            uuid.UUID   `gorm:"type:uuid;not null;index"`
}

type Block struct {
	BlockerPersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	BlockedPersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	BlockerUserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt        time.Time
}

type Mute struct {
	MuterPersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	MutedPersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt      time.Time
}

type ModerationCase struct {
	Base
	TargetType      ContentType `gorm:"size:16;not null;index:idx_case_target"`
	TargetID        uuid.UUID   `gorm:"type:uuid;not null;index:idx_case_target"`
	TargetPersonaID *uuid.UUID  `gorm:"type:uuid;index"`
	Severity        Severity    `gorm:"size:10;not null;index"`
	AISeverity      *Severity   `gorm:"size:10"`
	AIRationale     *string     `gorm:"type:text"`
	Status          CaseStatus  `gorm:"size:12;not null;index;default:open"`
	Source          string      `gorm:"size:16;not null;default:report"`
	ReportCount     int         `gorm:"not null;default:0"`
	AssigneeAdminID *uuid.UUID  `gorm:"type:uuid"`
	SLADueAt        time.Time   `gorm:"not null;index"`
}

type ModerationAction struct {
	Base
	CaseID       uuid.UUID  `gorm:"type:uuid;not null;index"`
	AdminID      uuid.UUID  `gorm:"type:uuid;not null"`
	Action       ModAction  `gorm:"size:16;not null"`
	TargetUserID *uuid.UUID `gorm:"type:uuid;index"`
	Until        *time.Time
	Note         string `gorm:"type:text"`
}

type Appeal struct {
	Base
	ActionID   uuid.UUID    `gorm:"type:uuid;not null;index"`
	UserID     uuid.UUID    `gorm:"type:uuid;not null;index"`
	TextMasked string       `gorm:"size:1000"`
	Status     AppealStatus `gorm:"size:12;not null;index;default:open"`
	DecidedBy  *uuid.UUID   `gorm:"type:uuid"`
	SLADueAt   time.Time    `gorm:"not null;index"`
}

type GrievanceTicket struct {
	Base
	UserID          *uuid.UUID `gorm:"type:uuid;index"`
	ContactEmailEnc []byte
	Category        string `gorm:"size:16;not null"`
	BodyEnc         []byte `gorm:"not null"`
	AcknowledgedAt  *time.Time
	ResolvedAt      *time.Time
	Status          CaseStatus `gorm:"size:12;not null;index;default:open"`
	AckDueAt        time.Time  `gorm:"not null;index"`
	ResolveDueAt    time.Time  `gorm:"not null;index"`
	Resolution      *string    `gorm:"type:text"`
	OverdueAlertAt  *time.Time
}

type TrustEvent struct {
	Base
	UserID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Kind   TrustEventKind `gorm:"size:20;not null"`
	Delta  int16          `gorm:"not null"`
	RefID  *uuid.UUID     `gorm:"type:uuid"`
}

func init() {
	register(&PIIVault{}, &PIIDetection{}, &SOSEvent{}, &Report{}, &Block{}, &Mute{}, &ModerationCase{}, &ModerationAction{}, &Appeal{}, &GrievanceTicket{}, &TrustEvent{})
}
