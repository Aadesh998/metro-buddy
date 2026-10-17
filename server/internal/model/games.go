package model

import (
	"time"

	"github.com/google/uuid"
)

// GameRoom is a lobby and, once started, one server-authoritative game.
// State is the engine's JSON; only the server reads it, players get a per-viewer view.
type GameRoom struct {
	Base
	Game          string     `gorm:"size:16;not null;index"`
	Visibility    string     `gorm:"size:8;not null;default:public"`
	Code          string     `gorm:"size:8;not null;uniqueIndex"`
	HostPersonaID uuid.UUID  `gorm:"type:uuid;not null"`
	MinPlayers    int16      `gorm:"not null"`
	MaxPlayers    int16      `gorm:"not null"`
	Status        string     `gorm:"size:10;not null;index;default:lobby"`
	State         string     `gorm:"type:text"`
	Version       int64      `gorm:"not null;default:0"`
	DeadlineAt    *time.Time `gorm:"index"`
	StartedAt     *time.Time
	EndedAt       *time.Time
	LastActiveAt  time.Time `gorm:"not null;index"`
}

type GamePlayer struct {
	RoomID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	Spectator bool      `gorm:"not null;default:false"`
	Ready     bool      `gorm:"not null;default:false"`
	Seat      int16     `gorm:"not null;default:0"`
	Score     int       `gorm:"not null;default:0"`
	JoinedAt  time.Time `gorm:"not null"`
}

func init() {
	register(&GameRoom{}, &GamePlayer{})
}
