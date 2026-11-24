package types

import (
	"database/sql/driver"
)

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
