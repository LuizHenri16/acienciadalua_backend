package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp is a PostgreSQL timestamp (without time zone) serialized as an
// ISO-8601 UTC string with millisecond precision.
type Timestamp struct {
	time.Time
}

// NewTimestamp normalizes a time to UTC.
func NewTimestamp(t time.Time) Timestamp {
	return Timestamp{Time: t.UTC()}
}

// AsTime returns the wrapped time.Time in UTC.
func (t Timestamp) AsTime() time.Time {
	return t.Time.UTC()
}

// Scan implements sql.Scanner.
func (t *Timestamp) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		t.Time = time.Time{}
		return nil
	case time.Time:
		t.Time = v
		return nil
	case string:
		return t.parse(v)
	case []byte:
		return t.parse(string(v))
	default:
		return fmt.Errorf("types.Timestamp: cannot scan %T", src)
	}
}

func (t *Timestamp) parse(s string) error {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("types.Timestamp: cannot parse %q", s)
}

// Value implements driver.Valuer.
func (t Timestamp) Value() (driver.Value, error) {
	return t.Time, nil
}

// MarshalJSON renders "YYYY-MM-DDTHH:MM:SS.mmmZ".
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.UTC().Format("2006-01-02T15:04:05.000Z"))
}

// UnmarshalJSON accepts ISO-8601 strings and unix milliseconds.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		t.Time = time.Time{}
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return t.parse(s)
	}

	var ms int64
	if err := json.Unmarshal(data, &ms); err != nil {
		return fmt.Errorf("types.Timestamp: invalid value %s", data)
	}
	t.Time = time.UnixMilli(ms).UTC()
	return nil
}
