// Package types holds the database value types shared between the sqlc layer
// and the HTTP layer.
//
// They define the JSON contract of the API:
//   - numeric values are serialized as JSON strings (e.g. "49.9");
//   - timestamps are serialized as ISO-8601 UTC strings with exactly
//     millisecond precision (e.g. "2026-10-24T16:33:00.000Z").
package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Decimal is a PostgreSQL numeric value serialized as a JSON string.
type Decimal struct {
	decimal.Decimal
}

// NewDecimal builds a Decimal from a string. Invalid input yields zero.
func NewDecimal(s string) Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}
	}
	return Decimal{Decimal: d}
}

// MustDecimal builds a Decimal and panics on invalid input. Useful in tests.
func MustDecimal(s string) Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return Decimal{Decimal: d}
}

// Scan implements sql.Scanner (used by pgx for numeric columns).
func (d *Decimal) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		d.Decimal = decimal.Zero
		return nil
	case string:
		return d.setFromString(v)
	case []byte:
		return d.setFromString(string(v))
	case float64:
		d.Decimal = decimal.NewFromFloat(v)
		return nil
	case int64:
		d.Decimal = decimal.NewFromInt(v)
		return nil
	default:
		return fmt.Errorf("types.Decimal: cannot scan %T", src)
	}
}

func (d *Decimal) setFromString(s string) error {
	parsed, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("types.Decimal: %w", err)
	}
	d.Decimal = parsed
	return nil
}

// Value implements driver.Valuer. Numeric is sent as text so no precision is
// ever lost on the way to PostgreSQL.
func (d Decimal) Value() (driver.Value, error) {
	return d.Decimal.String(), nil
}

// MarshalJSON renders the value as a JSON string.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Decimal.String())
}

// UnmarshalJSON accepts both JSON strings and numbers.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		d.Decimal = decimal.Zero
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return d.setFromString(s)
	}

	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("types.Decimal: invalid value %s", data)
	}
	d.Decimal = decimal.NewFromFloat(f)
	return nil
}

// NullDecimal is the nullable counterpart of Decimal.
type NullDecimal struct {
	Decimal Decimal
	Valid   bool
}

func (d *NullDecimal) Scan(src any) error {
	if src == nil {
		d.Valid = false
		return nil
	}
	if err := d.Decimal.Scan(src); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

func (d NullDecimal) Value() (driver.Value, error) {
	if !d.Valid {
		return nil, nil
	}
	return d.Decimal.Value()
}

func (d NullDecimal) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return d.Decimal.MarshalJSON()
}

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
