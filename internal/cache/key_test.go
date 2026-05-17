package cache

import (
	"net/http"
	"testing"
)

func TestKeyIncludesQuery(t *testing.T) {
	a := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "id=1"})
	b := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "id=2"})
	if a == b {
		t.Fatal("expected different keys for different queries")
	}
}

func TestKeySortsEquivalentQueries(t *testing.T) {
	a := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "b=2&a=1"})
	b := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "a=1&b=2"})
	if a != b {
		t.Fatal("expected reordered query params to produce the same key")
	}
}

func TestKeySortsRepeatedQueryValues(t *testing.T) {
	a := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "role=admin&role=user"})
	b := Key(KeyRequest{Method: http.MethodGet, Path: "/users", RawQuery: "role=user&role=admin"})
	if a != b {
		t.Fatal("expected reordered repeated query values to produce the same key")
	}
}

func TestKeyIncludesBodyHashForMutableMethods(t *testing.T) {
	tests := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			a := Key(KeyRequest{Method: method, Path: "/login", Body: []byte("A")})
			b := Key(KeyRequest{Method: method, Path: "/login", Body: []byte("B")})
			if a == b {
				t.Fatalf("expected different keys for different %s bodies", method)
			}
		})
	}
}

func TestKeyIgnoresBodyForGET(t *testing.T) {
	a := Key(KeyRequest{Method: http.MethodGet, Path: "/users", Body: []byte("A")})
	b := Key(KeyRequest{Method: http.MethodGet, Path: "/users", Body: []byte("B")})
	if a != b {
		t.Fatal("expected GET body to be ignored")
	}
}

func TestKeyIncludesConfiguredVaryHeaders(t *testing.T) {
	a := Key(KeyRequest{
		Method:      http.MethodGet,
		Path:        "/users",
		Headers:     http.Header{"Authorization": []string{"Bearer A"}},
		VaryHeaders: []string{"authorization"},
	})
	b := Key(KeyRequest{
		Method:      http.MethodGet,
		Path:        "/users",
		Headers:     http.Header{"Authorization": []string{"Bearer B"}},
		VaryHeaders: []string{"Authorization"},
	})
	if a == b {
		t.Fatal("expected configured Vary header value to affect cache key")
	}
}
