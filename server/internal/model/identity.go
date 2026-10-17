package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	SoftBase
	PhoneHash           []byte     `gorm:"uniqueIndex;not null"`
	PhoneEnc            []byte     `gorm:"not null"`
	Status              UserStatus `gorm:"size:20;not null;index;default:active"`
	BirthYear           *int16
	Gender              *Gender    `gorm:"size:20"`
	CityID              *uuid.UUID `gorm:"type:uuid"`
	VerifiedAt          *time.Time
	TrustScore          int        `gorm:"not null;default:50"`
	TrustTier           int16      `gorm:"not null;default:0;index"`
	Locale              string     `gorm:"size:8;not null;default:en"`
	LastActiveAt        *time.Time `gorm:"index"`
	SuspendedUntil      *time.Time
	DeletionRequestedAt *time.Time `gorm:"index"`
}

type Persona struct {
	SoftBase
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	CityID         uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_persona_city_alias"`
	Alias          string    `gorm:"size:40;not null"`
	AliasKey       string    `gorm:"size:40;not null;uniqueIndex:idx_persona_city_alias"`
	AvatarSeed     string    `gorm:"size:32;not null"`
	Bio            string    `gorm:"size:160"`
	AliasChangedAt *time.Time
	HomeStationID  *uuid.UUID `gorm:"type:uuid"`
	WorkStationID  *uuid.UUID `gorm:"type:uuid"`
	Intents        StringList `gorm:"type:text"`
	IsHost         bool       `gorm:"not null;default:false"`
}

type PersonaAliasHistory struct {
	Base
	PersonaID uuid.UUID `gorm:"type:uuid;not null;index"`
	Alias     string    `gorm:"size:40;not null;index"`
	UsedFrom  time.Time
	UsedTo    time.Time
}

type Device struct {
	Base
	UserID        uuid.UUID `gorm:"type:uuid;not null;index"`
	Platform      Platform  `gorm:"size:10"`
	PushToken     *string   `gorm:"type:text"`
	AppVersion    string    `gorm:"size:16"`
	AttestationOK *bool
	LastSeenAt    time.Time
}

type RefreshToken struct {
	Base
	UserID     uuid.UUID  `gorm:"type:uuid;not null;index"`
	DeviceID   *uuid.UUID `gorm:"type:uuid"`
	FamilyID   uuid.UUID  `gorm:"type:uuid;not null;index"`
	TokenHash  []byte     `gorm:"uniqueIndex;not null"`
	ExpiresAt  time.Time  `gorm:"not null"`
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID `gorm:"type:uuid"`
}

type Consent struct {
	Base
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index"`
	Purpose       ConsentPurpose `gorm:"size:24;not null"`
	Granted       bool           `gorm:"not null"`
	PolicyVersion string         `gorm:"size:16;not null"`
}

type TrustedContact struct {
	Base
	UserID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Name     string    `gorm:"size:60;not null"`
	PhoneEnc []byte    `gorm:"not null"`
}

type UserSettings struct {
	UserID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	MatchSameGenderOnly bool      `gorm:"not null;default:false"`
	MatchVerifiedOnly   bool      `gorm:"not null;default:false"`
	MatchAgeMin         *int16
	MatchAgeMax         *int16
	// Minutes after local midnight (IST).
	QuietHoursStart *int16
	QuietHoursEnd   *int16
	DMFrom          DMPolicy `gorm:"size:12;not null;default:requests"`
	NotifPrefs      BoolMap  `gorm:"type:text"`
	UpdatedAt       time.Time
}

func init() {
	register(&User{}, &Persona{}, &PersonaAliasHistory{}, &Device{}, &RefreshToken{}, &Consent{}, &TrustedContact{}, &UserSettings{})
}
