package identity

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

const DeletedAlias = "[deleted]"

// PublicPersona is what any other user may see. It never carries user_id, stations, gender or age.
type PublicPersona struct {
	ID         uuid.UUID `json:"id"`
	Alias      string    `json:"alias"`
	AvatarSeed string    `json:"avatar_seed"`
	Bio        string    `json:"bio"`
	Intents    []string  `json:"intents"`
	IsHost     bool      `json:"is_host"`
	Verified   bool      `json:"verified"`
	Vibes      []VibeDTO `json:"vibes"`
	Deleted    bool      `json:"deleted"`
}

type VibeDTO struct {
	Slug     string `json:"slug"`
	LabelEN  string `json:"label_en"`
	LabelHI  string `json:"label_hi"`
	Category string `json:"category"`
}

// Compact author reference embedded in posts, comments and messages.
type PersonaRef struct {
	ID         uuid.UUID `json:"id"`
	Alias      string    `json:"alias"`
	AvatarSeed string    `json:"avatar_seed"`
	Verified   bool      `json:"verified"`
	IsHost     bool      `json:"is_host"`
}

func DeletedRef(id uuid.UUID) PersonaRef { return PersonaRef{ID: id, Alias: DeletedAlias} }

type personaRow struct {
	ID         uuid.UUID
	Alias      string
	AvatarSeed string
	Bio        string
	Intents    model.StringList `gorm:"serializer:json"`
	IsHost     bool
	VerifiedAt *string
	DeletedAt  *string
}

func loadRows(tx *gorm.DB, ids []uuid.UUID) ([]personaRow, error) {
	var rows []personaRow
	if len(ids) == 0 {
		return rows, nil
	}
	// Identity domain is the only place allowed to join persona to user, here only for the verified badge.
	err := tx.Table("personas").
		Select("personas.id, personas.alias, personas.avatar_seed, personas.bio, personas.intents, personas.is_host, users.verified_at, personas.deleted_at").
		Joins("JOIN users ON users.id = personas.user_id").
		Where("personas.id IN ?", ids).Scan(&rows).Error
	return rows, err
}

func LoadRefs(tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]PersonaRef, error) {
	rows, err := loadRows(tx, uniq(ids))
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]PersonaRef, len(rows))
	for _, r := range rows {
		if r.DeletedAt != nil {
			out[r.ID] = DeletedRef(r.ID)
			continue
		}
		out[r.ID] = PersonaRef{ID: r.ID, Alias: r.Alias, AvatarSeed: r.AvatarSeed, Verified: r.VerifiedAt != nil, IsHost: r.IsHost}
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = DeletedRef(id)
		}
	}
	return out, nil
}

func LoadPublic(tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]PublicPersona, error) {
	rows, err := loadRows(tx, uniq(ids))
	if err != nil {
		return nil, err
	}
	vibes, err := LoadVibes(tx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]PublicPersona, len(rows))
	for _, r := range rows {
		if r.DeletedAt != nil {
			out[r.ID] = PublicPersona{ID: r.ID, Alias: DeletedAlias, Deleted: true, Intents: []string{}, Vibes: []VibeDTO{}}
			continue
		}
		out[r.ID] = PublicPersona{
			ID: r.ID, Alias: r.Alias, AvatarSeed: r.AvatarSeed, Bio: r.Bio, Intents: nonNil(r.Intents),
			IsHost: r.IsHost, Verified: r.VerifiedAt != nil, Vibes: nonNilVibes(vibes[r.ID]),
		}
	}
	return out, nil
}

func LoadVibes(tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID][]VibeDTO, error) {
	type row struct {
		PersonaID uuid.UUID
		VibeDTO
	}
	var rows []row
	if len(ids) == 0 {
		return map[uuid.UUID][]VibeDTO{}, nil
	}
	err := tx.Table("persona_vibes").
		Select("persona_vibes.persona_id, vibe_tags.slug, vibe_tags.label_en, vibe_tags.label_hi, vibe_tags.category").
		Joins("JOIN vibe_tags ON vibe_tags.id = persona_vibes.vibe_tag_id").
		Where("persona_vibes.persona_id IN ?", uniq(ids)).Order("vibe_tags.category, vibe_tags.slug").Scan(&rows).Error
	out := map[uuid.UUID][]VibeDTO{}
	for _, r := range rows {
		out[r.PersonaID] = append(out[r.PersonaID], r.VibeDTO)
	}
	return out, err
}

func nonNilVibes(v []VibeDTO) []VibeDTO {
	if v == nil {
		return []VibeDTO{}
	}
	return v
}

func uniq(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
