package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

func jsonValue(v any) (driver.Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func jsonScan(src any, dst any) error {
	switch s := src.(type) {
	case nil:
		return nil
	case string:
		return json.Unmarshal([]byte(s), dst)
	case []byte:
		return json.Unmarshal(s, dst)
	}
	return fmt.Errorf("cannot scan %T into JSON column", src)
}

func (l StringList) Value() (driver.Value, error) { return jsonValue(l) }
func (l *StringList) Scan(src any) error          { return jsonScan(src, l) }
func (l IDList) Value() (driver.Value, error)     { return jsonValue(l) }
func (l *IDList) Scan(src any) error              { return jsonScan(src, l) }
func (m JSONMap) Value() (driver.Value, error)    { return jsonValue(m) }
func (m *JSONMap) Scan(src any) error             { return jsonScan(src, m) }
func (m BoolMap) Value() (driver.Value, error)    { return jsonValue(m) }
func (m *BoolMap) Scan(src any) error             { return jsonScan(src, m) }
func (l MediaList) Value() (driver.Value, error)  { return jsonValue(l) }
func (l *MediaList) Scan(src any) error           { return jsonScan(src, l) }
func (w MatchWhy) Value() (driver.Value, error)   { return jsonValue(w) }
func (w *MatchWhy) Scan(src any) error            { return jsonScan(src, w) }
