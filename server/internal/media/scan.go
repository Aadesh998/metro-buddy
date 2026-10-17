package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Verdict int

const (
	Clear Verdict = iota
	Blocked
	// Needs a human: the classifier was unsure or isn't configured.
	Review
)

type Scanner interface {
	Scan(ctx context.Context, jpeg []byte) (Verdict, error)
}

// HTTPScanner posts the image to a classifier that returns {"nsfw": 0..1}.
type HTTPScanner struct {
	Endpoint string
	Client   *http.Client
}

func (s HTTPScanner) Scan(ctx context.Context, img []byte) (Verdict, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(img))
	req.Header.Set("Content-Type", "image/jpeg")
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return Review, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Review, fmt.Errorf("nsfw classifier status %d", resp.StatusCode)
	}
	var out struct {
		NSFW float64 `json:"nsfw"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Review, err
	}
	switch {
	case out.NSFW >= 0.85:
		return Blocked, nil
	case out.NSFW >= 0.4:
		return Review, nil
	}
	return Clear, nil
}

// ReviewAll sends every image to human review; used when no classifier is configured.
type ReviewAll struct{}

func (ReviewAll) Scan(context.Context, []byte) (Verdict, error) { return Review, nil }

type ClearAll struct{}

func (ClearAll) Scan(context.Context, []byte) (Verdict, error) { return Clear, nil }
