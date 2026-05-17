package cache

import (
	"net/http"
	"time"
)

type Entry struct {
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
	CreatedAt  time.Time   `json:"created_at"`
}

func (e Entry) DeepCopy() Entry {
	return Entry{
		StatusCode: e.StatusCode,
		Headers:    e.Headers.Clone(),
		Body:       append([]byte(nil), e.Body...),
		CreatedAt:  e.CreatedAt,
	}
}

func (e Entry) Expired(now time.Time, ttl time.Duration) bool {
	return now.Sub(e.CreatedAt) > ttl
}

func (e Entry) BodySize() int {
	return len(e.Body)
}
