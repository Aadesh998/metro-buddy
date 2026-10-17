package model

import (
	"time"

	"github.com/google/uuid"
)

type HostProfile struct {
	Base
	PersonaID  uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex"`
	Status     HostStatus `gorm:"size:10;not null;index;default:applied"`
	Tagline    string     `gorm:"size:80"`
	Pitch      string     `gorm:"size:500"`
	ApprovedBy *uuid.UUID `gorm:"type:uuid"`
}

type Show struct {
	SoftBase
	HostPersonaID uuid.UUID  `gorm:"type:uuid;not null;index"`
	CityID        *uuid.UUID `gorm:"type:uuid;index"`
	Title         string     `gorm:"size:60;not null"`
	Description   string     `gorm:"size:300"`
	TopicTags     StringList `gorm:"type:text"`
	ScheduleRRule *string    `gorm:"size:120"`
	FollowerCount int        `gorm:"not null;default:0"`
}

type ShowFollower struct {
	ShowID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	CreatedAt time.Time
}

type ShowEpisode struct {
	Base
	ShowID         uuid.UUID `gorm:"type:uuid;not null;index"`
	Title          string    `gorm:"size:80;not null"`
	ScheduledAt    time.Time `gorm:"not null;index"`
	StartedAt      *time.Time
	EndedAt        *time.Time
	Status         EpisodeStatus `gorm:"size:10;not null;index;default:scheduled"`
	LiveKitRoom    string        `gorm:"size:64;not null;uniqueIndex"`
	PeakListeners  int           `gorm:"not null;default:0"`
	ReminderSentAt *time.Time
	// Set while on-air audio is force-muted after a report spike.
	AutoMutedAt *time.Time
}

type EpisodeParticipant struct {
	EpisodeID uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	Role      StageRole `gorm:"size:10;not null"`
	JoinedAt  time.Time `gorm:"not null"`
	LeftAt    *time.Time
}

type CallInRequest struct {
	Base
	EpisodeID   uuid.UUID    `gorm:"type:uuid;not null;index"`
	PersonaID   uuid.UUID    `gorm:"type:uuid;not null;index"`
	TopicMasked string       `gorm:"size:80;not null"`
	Status      CallInStatus `gorm:"size:10;not null;index;default:queued"`
	OnAirAt     *time.Time
	OffAirAt    *time.Time
	MaxSeconds  int  `gorm:"not null;default:480"`
	VoiceMasked bool `gorm:"not null;default:false"`
}

type EpisodeQuestion struct {
	Base
	EpisodeID  uuid.UUID     `gorm:"type:uuid;not null;index"`
	PersonaID  uuid.UUID     `gorm:"type:uuid;not null"`
	TextMasked string        `gorm:"size:200;not null"`
	PIIFlags   int16         `gorm:"not null;default:0"`
	Status     ContentStatus `gorm:"size:20;not null;default:visible"`
	Answered   bool          `gorm:"not null;default:false"`
}

func init() {
	register(&HostProfile{}, &Show{}, &ShowFollower{}, &ShowEpisode{}, &EpisodeParticipant{}, &CallInRequest{}, &EpisodeQuestion{})
}
