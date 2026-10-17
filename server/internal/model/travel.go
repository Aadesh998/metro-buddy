package model

import (
	"time"

	"github.com/google/uuid"
)

type CommutePlan struct {
	SoftBase
	PersonaID     uuid.UUID `gorm:"type:uuid;not null;index"`
	FromStationID uuid.UUID `gorm:"type:uuid;not null"`
	ToStationID   uuid.UUID `gorm:"type:uuid;not null"`
	// Minutes after local midnight (IST).
	DepartWindowStart int16  `gorm:"not null"`
	DepartWindowEnd   int16  `gorm:"not null"`
	Weekdays          int16  `gorm:"not null"`
	PathStationIDs    IDList `gorm:"serializer:json;type:text"`
	Active            bool   `gorm:"not null;default:true;index"`
}

type TravelIntent struct {
	Base
	PersonaID      uuid.UUID    `gorm:"type:uuid;not null;index"`
	CommutePlanID  *uuid.UUID   `gorm:"type:uuid;index"`
	FromStationID  uuid.UUID    `gorm:"type:uuid;not null"`
	ToStationID    uuid.UUID    `gorm:"type:uuid;not null"`
	DepartAt       time.Time    `gorm:"not null;index"`
	ExpiresAt      time.Time    `gorm:"not null;index"`
	PathStationIDs IDList       `gorm:"serializer:json;type:text"`
	LineIDs        IDList       `gorm:"serializer:json;type:text"`
	Mood           TravelMood   `gorm:"size:8;not null;default:either"`
	Status         IntentStatus `gorm:"size:10;not null;index;default:active"`
}

type MatchWhy struct {
	Chips        []string `json:"chips"`
	SharedVibes  []string `json:"shared_vibes"`
	OverlapCount int      `json:"overlap_count"`
	MinutesApart int      `json:"minutes_apart"`
}

type Match struct {
	Base
	IntentAID            uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_match_pair"`
	IntentBID            uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_match_pair"`
	PersonaAID           uuid.UUID       `gorm:"type:uuid;not null;index"`
	PersonaBID           uuid.UUID       `gorm:"type:uuid;not null;index"`
	Score                float64         `gorm:"not null"`
	Why                  MatchWhy        `gorm:"serializer:json;type:text"`
	OverlapFromStationID uuid.UUID       `gorm:"type:uuid;not null"`
	OverlapToStationID   uuid.UUID       `gorm:"type:uuid;not null"`
	AStatus              MatchSideStatus `gorm:"size:10;not null;default:pending"`
	BStatus              MatchSideStatus `gorm:"size:10;not null;default:pending"`
	Status               MatchStatus     `gorm:"size:10;not null;index;default:proposed"`
	ExpiresAt            time.Time       `gorm:"not null;index"`
}

type Trip struct {
	Base
	MatchID        *uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	CrewID         *uuid.UUID `gorm:"type:uuid;index"`
	MeetStationID  uuid.UUID  `gorm:"type:uuid;not null"`
	MeetPlatform   string     `gorm:"size:40"`
	MeetCoach      string     `gorm:"size:20"`
	MeetAt         time.Time  `gorm:"not null;index"`
	Status         TripStatus `gorm:"size:12;not null;index;default:planned"`
	ShareToken     *string    `gorm:"size:32;uniqueIndex"`
	ShareExpiresAt *time.Time
}

type TripParticipant struct {
	TripID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID     uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	ToStationID   uuid.UUID `gorm:"type:uuid"`
	CheckedInAt   *time.Time
	NoShow        bool       `gorm:"not null;default:false"`
	LastStationID *uuid.UUID `gorm:"type:uuid"`
}

type TripFeedback struct {
	Base
	TripID        uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_feedback_once"`
	FromPersonaID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_feedback_once"`
	ToPersonaID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_feedback_once"`
	Thumbs        int16     `gorm:"not null"`
	FeltSafe      bool      `gorm:"not null"`
	SafetyFlag    bool      `gorm:"not null;default:false"`
	NoteMasked    string    `gorm:"size:300"`
}

type Crew struct {
	SoftBase
	Name               string    `gorm:"size:40;not null"`
	LineID             uuid.UUID `gorm:"type:uuid;not null;index"`
	DepartWindowStart  int16     `gorm:"not null"`
	Weekdays           int16     `gorm:"not null"`
	MemberCap          int16     `gorm:"not null;default:6"`
	CreatedByPersonaID uuid.UUID `gorm:"type:uuid;not null"`
}

type CrewInvite struct {
	Base
	CrewID        uuid.UUID     `gorm:"type:uuid;not null;index"`
	FromPersonaID uuid.UUID     `gorm:"type:uuid;not null"`
	ToPersonaID   uuid.UUID     `gorm:"type:uuid;not null;index"`
	Status        RequestStatus `gorm:"size:10;not null;default:pending"`
}

func init() {
	register(&CommutePlan{}, &TravelIntent{}, &Match{}, &Trip{}, &TripParticipant{}, &TripFeedback{}, &Crew{}, &CrewInvite{})
}
