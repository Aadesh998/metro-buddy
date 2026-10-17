package model

import "github.com/google/uuid"

type City struct {
	Base
	Name   string `gorm:"size:40;not null;uniqueIndex"`
	Code   string `gorm:"size:8;not null;uniqueIndex"`
	IsLive bool   `gorm:"not null;default:false"`
}

type MetroLine struct {
	Base
	CityID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Name        string    `gorm:"size:40;not null"`
	ColorHex    string    `gorm:"size:7;not null"`
	GTFSRouteID string    `gorm:"size:32;not null;uniqueIndex"`
}

type Station struct {
	Base
	CityID          uuid.UUID `gorm:"type:uuid;not null;index"`
	Name            string    `gorm:"size:80;not null;index"`
	Code            string    `gorm:"size:16;not null;uniqueIndex"`
	Lat             float64
	Lng             float64
	IsInterchange   bool    `gorm:"not null;default:false"`
	WomensCoachNote *string `gorm:"size:120"`
}

type LineStation struct {
	LineID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	StationID    uuid.UUID `gorm:"type:uuid;primaryKey;index"`
	Seq          int16     `gorm:"not null"`
	PlatformHint *string   `gorm:"size:40"`
}

type VibeTag struct {
	Base
	Slug     string `gorm:"size:32;not null;uniqueIndex"`
	LabelEN  string `gorm:"size:40;not null"`
	LabelHI  string `gorm:"size:40;not null"`
	Category string `gorm:"size:16;not null;index"`
}

type PersonaVibe struct {
	PersonaID uuid.UUID `gorm:"type:uuid;primaryKey"`
	VibeTagID uuid.UUID `gorm:"type:uuid;primaryKey;index"`
}

func init() {
	register(&City{}, &MetroLine{}, &Station{}, &LineStation{}, &VibeTag{}, &PersonaVibe{})
}
