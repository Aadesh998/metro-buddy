package pii

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

const VaultTTL = 90 * 24 * time.Hour

type Pipeline struct {
	DB     *gorm.DB
	Box    *secure.Box
	Audit  *audit.Recorder
	Second *SecondPass
	Now    func() time.Time
}

func (p *Pipeline) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now().UTC()
}

// AfterCommit queues the LLM second pass; call it once the content row is committed.
func (p *Pipeline) AfterCommit(ctx context.Context, ct model.ContentType, id uuid.UUID) {
	p.Second.Enqueue(ctx, ct, id)
}

// Process masks raw UGC and, inside tx, vaults the encrypted raw text and records what was
// masked. Raw is vaulted even when regex finds nothing, because the async LLM pass may mask
// more later. The returned masked text is the only version to store in content tables.
func (p *Pipeline) Process(tx *gorm.DB, ct model.ContentType, id uuid.UUID, raw string, opt Options) (Result, error) {
	r := Mask(raw, opt)
	if err := p.vault(tx, ct, id, raw); err != nil {
		return r, err
	}
	for _, k := range r.Kinds {
		if err := tx.Create(&model.PIIDetection{ContentType: ct, ContentID: id, Kind: k, Source: model.DetectorRegex}).Error; err != nil {
			return r, err
		}
	}
	return r, nil
}

func (p *Pipeline) vault(tx *gorm.DB, ct model.ContentType, id uuid.UUID, raw string) error {
	return tx.Create(&model.PIIVault{
		ContentType: ct, ContentID: id, RawEnc: p.Box.SealString(raw),
		KeyVersion: p.Box.KeyVersion(), ExpiresAt: p.now().Add(VaultTTL),
	}).Error
}

func (p *Pipeline) Purge(ctx context.Context) error {
	return p.DB.WithContext(ctx).Where("expires_at < ?", p.now()).Delete(&model.PIIVault{}).Error
}

type VaultEntry struct {
	ContentType model.ContentType `json:"content_type"`
	ContentID   uuid.UUID         `json:"content_id"`
	Raw         string            `json:"raw"`
	ExpiresAt   time.Time         `json:"expires_at"`
}

// ReadVault is for trust & safety leads only (enforced by the caller's RBAC); every read is audited.
func (p *Pipeline) ReadVault(ctx context.Context, adminID uuid.UUID, ip string, ct model.ContentType, id uuid.UUID, reason string) ([]VaultEntry, error) {
	var rows []model.PIIVault
	if err := p.DB.WithContext(ctx).Where("content_type = ? AND content_id = ? AND expires_at > ?", ct, id, p.now()).Find(&rows).Error; err != nil {
		return nil, err
	}
	if err := p.Audit.Log(p.DB.WithContext(ctx), model.ActorAdmin, &adminID, "pii_vault.read", string(ct), &id,
		model.JSONMap{"reason": reason, "entries": len(rows)}); err != nil {
		return nil, err
	}
	p.Audit.Security(ctx, model.SecVaultAccess, nil, ip, model.JSONMap{"admin_id": adminID.String(), "content_type": string(ct)})
	out := make([]VaultEntry, 0, len(rows))
	for _, r := range rows {
		raw, err := p.Box.OpenString(r.RawEnc)
		if err != nil {
			return nil, err
		}
		out = append(out, VaultEntry{ContentType: r.ContentType, ContentID: r.ContentID, Raw: raw, ExpiresAt: r.ExpiresAt})
	}
	return out, nil
}
