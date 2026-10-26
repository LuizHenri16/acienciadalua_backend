package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterBlocksAfterLimit(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/user/signin", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	assert.Equal(t, http.StatusOK, request().Code)
	assert.Equal(t, http.StatusOK, request().Code)

	blocked := request()
	require.Equal(t, http.StatusTooManyRequests, blocked.Code)
	assert.NotEmpty(t, blocked.Header().Get("Retry-After"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(blocked.Body.Bytes(), &body))
	assert.Equal(t, "ThrottlerException: Too Many Requests", body["message"])
	assert.EqualValues(t, 429, body["statusCode"])
}
