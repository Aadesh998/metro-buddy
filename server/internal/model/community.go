package model

import (
	"time"

	"github.com/google/uuid"
)

type Bowl struct {
	SoftBase
	CityID             *uuid.UUID     `gorm:"type:uuid;index"`
	Kind               BowlKind       `gorm:"size:16;not null;index"`
	Slug               string         `gorm:"size:48;not null;uniqueIndex"`
	Name               string         `gorm:"size:60;not null"`
	Description        string         `gorm:"size:300"`
	LineID             *uuid.UUID     `gorm:"type:uuid;index"`
	StationID          *uuid.UUID     `gorm:"type:uuid;index"`
	Visibility         BowlVisibility `gorm:"size:16;not null;default:public"`
	CreatedByPersonaID *uuid.UUID     `gorm:"type:uuid"`
	MemberCount        int            `gorm:"not null;default:0"`
	RulesMD            string         `gorm:"type:text"`
	PinnedPostID       *uuid.UUID     `gorm:"type:uuid"`
}

type BowlMember struct {
	BowlID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID  uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	Role       BowlRole  `gorm:"size:12;not null;default:member"`
	JoinedAt   time.Time `gorm:"not null"`
	MutedUntil *time.Time
}

type Media struct {
	Key    string `json:"key"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
	Mime   string `json:"mime"`
	// Images stay hidden until the NSFW scan clears them.
	Cleared bool `json:"cleared"`
}

type MediaList []Media

type Post struct {
	SoftBase
	BowlID          uuid.UUID     `gorm:"type:uuid;not null;index"`
	AuthorPersonaID uuid.UUID     `gorm:"type:uuid;not null;index"`
	Type            PostType      `gorm:"size:8;not null"`
	Title           string        `gorm:"size:140;not null"`
	BodyMasked      string        `gorm:"type:text"`
	Flair           *string       `gorm:"size:24"`
	Media           MediaList     `gorm:"type:text"`
	PIIFlags        int16         `gorm:"not null;default:0"`
	Status          ContentStatus `gorm:"size:20;not null;index;default:visible"`
	Score           int           `gorm:"not null;default:0"`
	CommentCount    int           `gorm:"not null;default:0"`
	HotRank         float64       `gorm:"not null;default:0;index"`
	IsPrompt        bool          `gorm:"not null;default:false"`
	LockedAt        *time.Time
}

type Poll struct {
	PostID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	ClosesAt time.Time `gorm:"not null"`
	Multi    bool      `gorm:"not null;default:false"`
}

type PollOption struct {
	Base
	PostID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Label     string    `gorm:"size:60;not null"`
	Position  int16     `gorm:"not null"`
	VoteCount int       `gorm:"not null;default:0"`
}

type PollVote struct {
	OptionID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	PostID    uuid.UUID `gorm:"type:uuid;not null;index"`
}

type Comment struct {
	SoftBase
	PostID          uuid.UUID     `gorm:"type:uuid;not null;index"`
	ParentID        *uuid.UUID    `gorm:"type:uuid;index"`
	AuthorPersonaID uuid.UUID     `gorm:"type:uuid;not null;index"`
	BodyMasked      string        `gorm:"type:text;not null"`
	PIIFlags        int16         `gorm:"not null;default:0"`
	Depth           int16         `gorm:"not null;default:0"`
	Status          ContentStatus `gorm:"size:20;not null;default:visible"`
	Score           int           `gorm:"not null;default:0"`
}

type Vote struct {
	PersonaID  uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TargetType VoteTarget `gorm:"size:8;primaryKey"`
	TargetID   uuid.UUID  `gorm:"type:uuid;primaryKey;index"`
	Value      int16      `gorm:"not null"`
}

type DailyPrompt struct {
	Base
	CityID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_prompt_city_day"`
	Day    string    `gorm:"size:10;not null;uniqueIndex:idx_prompt_city_day"`
	Text   string    `gorm:"size:200;not null"`
	PostID uuid.UUID `gorm:"type:uuid;not null"`
}

func init() {
	register(&Bowl{}, &BowlMember{}, &Post{}, &Poll{}, &PollOption{}, &PollVote{}, &Comment{}, &Vote{}, &DailyPrompt{})
}
