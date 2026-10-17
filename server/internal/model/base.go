package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"not null;index"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = NewID()
	}
	return nil
}

type SoftBase struct {
	Base
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func NewID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

func All() []any {
	return registry
}

var registry []any

func register(m ...any) {
	registry = append(registry, m...)
}
