package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecimalJSONMatchesPrisma(t *testing.T) {
	value := MustDecimal("49.9000")

	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	assert.JSONEq(t, `"49.9"`, string(encoded))

	var decoded Decimal
	require.NoError(t, json.Unmarshal([]byte(`"12.340"`), &decoded))
	assert.Equal(t, "12.34", decoded.String())

	require.NoError(t, json.Unmarshal([]byte(`12.5`), &decoded))
	assert.Equal(t, "12.5", decoded.String())
}

func TestDecimalScan(t *testing.T) {
	var value Decimal
	require.NoError(t, value.Scan("59.500000000000000000000000000000"))
	assert.Equal(t, "59.5", value.String())

	driverValue, err := value.Value()
	require.NoError(t, err)
	assert.Equal(t, "59.5", driverValue)
}

func TestTimestampJSONMatchesPrisma(t *testing.T) {
	value := NewTimestamp(time.Date(2026, 10, 24, 16, 33, 0, 0, time.UTC))

	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-24T16:33:00.000Z"`, string(encoded))

	var decoded Timestamp
	require.NoError(t, json.Unmarshal([]byte(`"2026-10-24T16:33:00.000Z"`), &decoded))
	assert.Equal(t, value.AsTime(), decoded.AsTime())
}

func TestTimestampScan(t *testing.T) {
	source := time.Date(2026, 10, 24, 13, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	var value Timestamp
	require.NoError(t, value.Scan(source))

	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	assert.Equal(t, `"2026-10-24T16:00:00.000Z"`, string(encoded))
}
