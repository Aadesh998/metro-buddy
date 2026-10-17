package safety

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

// ExcludeBlocked hides rows whose persona column belongs to someone the viewer blocked
// or who blocked the viewer. Blocks are bidirectional and apply to every list.
func ExcludeBlocked(viewer uuid.UUID, col string) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where(col+" NOT IN (SELECT blocked_persona_id FROM blocks WHERE blocker_persona_id = ?)", viewer).
			Where(col+" NOT IN (SELECT blocker_persona_id FROM blocks WHERE blocked_persona_id = ?)", viewer)
	}
}

// ExcludeMuted is one-way: the viewer stops seeing the muted persona, who notices nothing.
func ExcludeMuted(viewer uuid.UUID, col string) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Where(col+" NOT IN (SELECT muted_persona_id FROM mutes WHERE muter_persona_id = ?)", viewer)
	}
}

// Hidden applies both block and mute filters; use for feeds and lists.
func Hidden(viewer uuid.UUID, col string) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Scopes(ExcludeBlocked(viewer, col), ExcludeMuted(viewer, col))
	}
}

func IsBlocked(ctx context.Context, db *gorm.DB, a, b uuid.UUID) (bool, error) {
	if a == b {
		return false, nil
	}
	var n int64
	err := db.WithContext(ctx).Model(&model.Block{}).
		Where("(blocker_persona_id = ? AND blocked_persona_id = ?) OR (blocker_persona_id = ? AND blocked_persona_id = ?)", a, b, b, a).
		Count(&n).Error
	return n > 0, err
}

// BlockedSet returns every persona hidden from viewer by a block in either direction.
func BlockedSet(ctx context.Context, db *gorm.DB, viewer uuid.UUID) (map[uuid.UUID]bool, error) {
	var rows []model.Block
	if err := db.WithContext(ctx).Where("blocker_persona_id = ? OR blocked_persona_id = ?", viewer, viewer).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		if r.BlockerPersonaID == viewer {
			out[r.BlockedPersonaID] = true
		} else {
			out[r.BlockerPersonaID] = true
		}
	}
	return out, nil
}

func Visible(db *gorm.DB) func(ctx context.Context, viewer, target uuid.UUID) (bool, error) {
	return func(ctx context.Context, viewer, target uuid.UUID) (bool, error) {
		b, err := IsBlocked(ctx, db, viewer, target)
		return !b, err
	}
}
