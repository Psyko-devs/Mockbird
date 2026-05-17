package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

func Key(method, path, rawQuery string, body []byte) string {
	var bodyHash string
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		sum := sha256.Sum256(body)
		bodyHash = hex.EncodeToString(sum[:])
	}

	sum := sha256.Sum256([]byte(method + "\n" + path + "\n" + rawQuery + "\n" + bodyHash))
	return hex.EncodeToString(sum[:])
}
