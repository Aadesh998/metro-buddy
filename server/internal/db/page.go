package db

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var ErrBadCursor = errors.New("invalid cursor")

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c Cursor) Encode() string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrBadCursor
	}
	ts, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return nil, ErrBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, ErrBadCursor
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrBadCursor
	}
	return &Cursor{CreatedAt: t, ID: u}, nil
}

func ClampLimit(n int) int {
	if n <= 0 {
		return DefaultLimit
	}
	return min(n, MaxLimit)
}

// Newest first on (created_at, id). table qualifies columns when the query joins.
func PageDesc(q *gorm.DB, table, cursor string, limit int) (*gorm.DB, int, error) {
	c, err := DecodeCursor(cursor)
	if err != nil {
		return nil, 0, err
	}
	col := func(n string) string {
		if table == "" {
			return n
		}
		return table + "." + n
	}
	limit = ClampLimit(limit)
	if c != nil {
		q = q.Where("("+col("created_at")+" < ? OR ("+col("created_at")+" = ? AND "+col("id")+" < ?))", c.CreatedAt, c.CreatedAt, c.ID)
	}
	return q.Order(col("created_at") + " DESC").Order(col("id") + " DESC").Limit(limit + 1), limit, nil
}

// Trim fetches limit+1 rows; the extra row only signals that another page exists.
func Trim[T any](rows []T, limit int, key func(T) Cursor) ([]T, string) {
	if len(rows) <= limit {
		return rows, ""
	}
	rows = rows[:limit]
	return rows, key(rows[len(rows)-1]).Encode()
}
