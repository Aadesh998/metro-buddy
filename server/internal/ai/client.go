package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

const (
	ModelHaiku  = "claude-haiku-5-5"
	ModelSonnet = "claude-sonnet-5-5"
)

// Micro-USD per token (= USD per million tokens).
var pricing = map[string]struct{ in, out float64 }{
	ModelHaiku:  {0.10, 0.50},
	ModelSonnet: {2.00, 10.00},
}

var (
	ErrDisabled = errors.New("ai disabled: no ANTHROPIC_API_KEY")
	ErrRefused  = errors.New("ai declined the request")
	ErrTrunc    = errors.New("ai output truncated")
)

type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	DB      *gorm.DB
	Log     *slog.Logger
}

func (c *Client) Enabled() bool { return c != nil && c.APIKey != "" }

type Request struct {
	Purpose   model.AIPurpose
	Model     string
	System    string
	User      string
	MaxTokens int
	Effort    string
	// JSON schema for structured output; nil for free text.
	Schema map[string]any
	UserID *uuid.UUID
}

type apiResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete sends one Messages API call and returns the concatenated text output.
func (c *Client) Complete(ctx context.Context, r Request) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}
	body := map[string]any{
		"model":      r.Model,
		"max_tokens": r.MaxTokens,
		"system":     r.System,
		"messages":   []map[string]any{{"role": "user", "content": r.User}},
	}
	oc := map[string]any{}
	if r.Effort != "" {
		oc["effort"] = r.Effort
	}
	if r.Schema != nil {
		oc["format"] = map[string]any{"type": "json_schema", "schema": r.Schema}
	}
	if len(oc) > 0 {
		body["output_config"] = oc
	}
	b, _ := json.Marshal(body)
	base := c.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic status %d", resp.StatusCode)
	}
	var out apiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("anthropic decode: %w", err)
	}
	c.record(r, out.Usage.InputTokens, out.Usage.OutputTokens)

	switch out.StopReason {
	case "refusal":
		return "", ErrRefused
	case "max_tokens":
		return "", ErrTrunc
	}
	var text bytes.Buffer
	for _, blk := range out.Content {
		if blk.Type == "text" {
			text.WriteString(blk.Text)
		}
	}
	return text.String(), nil
}

func (c *Client) record(r Request, in, out int) {
	p := pricing[r.Model]
	cost := int64(float64(in)*p.in + float64(out)*p.out)
	if err := c.DB.Create(&model.AIUsage{
		Purpose: r.Purpose, Model: r.Model, InputTokens: in, OutputTokens: out, CostMicrosUSD: cost, UserID: r.UserID,
	}).Error; err != nil && c.Log != nil {
		c.Log.Error("ai usage write failed", "err", err.Error())
	}
}

// UsedToday counts calls for a purpose by one user since UTC midnight, for per-user caps.
func (c *Client) UsedToday(ctx context.Context, purpose model.AIPurpose, userID uuid.UUID) (int64, error) {
	var n int64
	start := time.Now().UTC().Truncate(24 * time.Hour)
	err := c.DB.WithContext(ctx).Model(&model.AIUsage{}).
		Where("purpose = ? AND user_id = ? AND created_at >= ?", purpose, userID, start).Count(&n).Error
	return n, err
}
