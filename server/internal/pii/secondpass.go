package pii

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/model"
)

const llmQueue = "pii:llm"

// SecondPass asks an LLM to find PII the regex masker can't see (spelled-out or split-up
// numbers, "at gmail dot com", spaced-out handles) and masks it after the fact.
type SecondPass struct {
	DB    *gorm.DB
	Redis *redis.Client
	AI    *ai.Client
	Log   *slog.Logger
	// Fired when masked text changed, so clients can replace what they already rendered.
	OnUpdated func(ct model.ContentType, id uuid.UUID, masked string)
}

type job struct {
	CT model.ContentType `json:"ct"`
	ID uuid.UUID         `json:"id"`
}

func (s *SecondPass) Enqueue(ctx context.Context, ct model.ContentType, id uuid.UUID) {
	if s == nil || !s.AI.Enabled() {
		return
	}
	b, _ := json.Marshal(job{ct, id})
	if err := s.Redis.LPush(ctx, llmQueue, b).Err(); err != nil {
		s.Log.Error("pii queue push failed", "err", err.Error())
	}
}

func (s *SecondPass) Run(ctx context.Context) {
	for ctx.Err() == nil {
		res, err := s.Redis.BRPop(ctx, 5*time.Second, llmQueue).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) && ctx.Err() == nil {
				time.Sleep(time.Second)
			}
			continue
		}
		var j job
		if json.Unmarshal([]byte(res[1]), &j) != nil {
			continue
		}
		if err := s.Process(ctx, j.CT, j.ID); err != nil {
			s.Log.Warn("pii second pass failed", "content_type", string(j.CT), "err", err.Error())
		}
	}
}

type target struct {
	table, textCol, authorCol string
}

var targets = map[model.ContentType]target{
	model.CTMessage: {"messages", "body_masked", "sender_persona_id"},
	model.CTPost:    {"posts", "body_masked", "author_persona_id"},
	model.CTComment: {"comments", "body_masked", "author_persona_id"},
}

var tokenFor = map[model.PIIKind]string{
	model.PIIPhone: TokPhone, model.PIIEmail: TokEmail, model.PIIUPI: TokUPI, model.PIIAadhaar: TokID,
	model.PIIPAN: TokID, model.PIIHandle: TokHandle, model.PIIURL: TokLink, model.PIIAddress: "[address hidden]",
}

const systemPrompt = `You find personal contact or identity details hidden in short messages from an anonymous Indian commuter app.
Users try to share these despite an automatic filter, so look for obfuscated forms in English, Hindi and Hinglish:
- phone numbers spelled out or split ("nine eight seven six...", "nau aath saat", "98 76 five four", digits spread across words)
- emails written with words ("riya at gmail dot com", "riya g mail")
- UPI IDs, Aadhaar or PAN numbers written in words or with odd spacing
- social handles spelled with spaces or symbols ("r i y a _ 2 3 on insta", "find me on the gram as riya underscore 23")
- links written with words ("riyablog dot in")
- precise home or work addresses (flat or house number with street or society name)
Text already replaced by tokens like [phone hidden] is safe; ignore it.
Do not flag metro station names, line names, times, prices, counts, or public places.
Return every finding as the exact substring copied from the message. Return an empty list if there is nothing.`

var findingSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"findings"},
	"properties": map[string]any{
		"findings": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"text", "kind"},
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
					"kind": map[string]any{"type": "string", "enum": []string{"phone", "email", "upi", "aadhaar", "pan", "handle", "url", "address"}},
				},
			},
		},
	},
}

func (s *SecondPass) Process(ctx context.Context, ct model.ContentType, id uuid.UUID) error {
	t, ok := targets[ct]
	if !ok {
		return nil
	}
	db := s.DB.WithContext(ctx)
	var row struct {
		Text   string
		Author uuid.UUID
	}
	if err := db.Table(t.table).Select(t.textCol+" AS text, "+t.authorCol+" AS author").Where("id = ?", id).Take(&row).Error; err != nil {
		return err
	}
	if strings.TrimSpace(row.Text) == "" {
		return nil
	}
	// Only content whose author allowed AI processing ever leaves our servers.
	var userID uuid.UUID
	var ids []uuid.UUID
	if err := db.Model(&model.Persona{}).Unscoped().Where("id = ?", row.Author).Limit(1).Pluck("user_id", &ids).Error; err != nil || len(ids) == 0 {
		return err
	}
	userID = ids[0]
	var cons []model.Consent
	if err := db.Where("user_id = ? AND purpose = ?", userID, model.ConsentAI).Order("created_at DESC, id DESC").Limit(1).Find(&cons).Error; err != nil {
		return err
	}
	if len(cons) == 0 || !cons[0].Granted {
		return nil
	}

	out, err := s.AI.Complete(ctx, ai.Request{
		Purpose: model.AIPIISecondPass, Model: ai.ModelHaiku, Effort: "low", MaxTokens: 1024,
		System: systemPrompt, User: row.Text, Schema: findingSchema, UserID: &userID,
	})
	if err != nil {
		return err
	}
	var res struct {
		Findings []struct {
			Text string        `json:"text"`
			Kind model.PIIKind `json:"kind"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		return err
	}

	var masked string
	changed := false
	err = db.Transaction(func(tx *gorm.DB) error {
		var cur struct{ Text string }
		if err := tx.Table(t.table).Select(t.textCol+" AS text").Where("id = ?", id).Take(&cur).Error; err != nil {
			return err
		}
		masked = cur.Text
		var flags int16
		seen := map[model.PIIKind]bool{}
		for _, f := range res.Findings {
			tok, ok := tokenFor[f.Kind]
			// Reject findings that aren't verbatim or would erase almost nothing meaningful.
			if !ok || len(strings.TrimSpace(f.Text)) < 4 || strings.Contains(f.Text, "[") || !strings.Contains(masked, f.Text) {
				continue
			}
			masked = strings.ReplaceAll(masked, f.Text, tok)
			flags |= Flag(f.Kind)
			changed = true
			if !seen[f.Kind] {
				seen[f.Kind] = true
				if err := tx.Create(&model.PIIDetection{ContentType: ct, ContentID: id, Kind: f.Kind, Source: model.DetectorLLM}).Error; err != nil {
					return err
				}
			}
		}
		if !changed {
			return nil
		}
		return tx.Table(t.table).Where("id = ?", id).Updates(map[string]any{
			t.textCol:   masked,
			"pii_flags": gorm.Expr("pii_flags | ?", flags),
		}).Error
	})
	if err != nil {
		return err
	}
	if changed && s.OnUpdated != nil {
		s.OnUpdated(ct, id, masked)
	}
	return nil
}
