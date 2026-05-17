package cache

import (
	"net/http"
	"testing"
)

func TestKeyIncludesQuery(t *testing.T) {
	a := Key(http.MethodGet, "/users", "id=1", nil)
	b := Key(http.MethodGet, "/users", "id=2", nil)
	if a == b {
		t.Fatal("expected different keys for different queries")
	}
}

func TestKeyIncludesBodyHashForMutableMethods(t *testing.T) {
	tests := []string{http.MethodPost, http.MethodPut, http.MethodPatch}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			a := Key(method, "/login", "", []byte("A"))
			b := Key(method, "/login", "", []byte("B"))
			if a == b {
				t.Fatalf("expected different keys for different %s bodies", method)
			}
		})
	}
}

func TestKeyIgnoresBodyForGET(t *testing.T) {
	a := Key(http.MethodGet, "/users", "", []byte("A"))
	b := Key(http.MethodGet, "/users", "", []byte("B"))
	if a != b {
		t.Fatal("expected GET body to be ignored")
	}
}
