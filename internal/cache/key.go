package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
)

type KeyRequest struct {
	Method      string
	Path        string
	RawQuery    string
	Body        []byte
	Headers     http.Header
	VaryHeaders []string
}

func Key(req KeyRequest) string {
	material := strings.Builder{}
	material.WriteString(strings.ToUpper(req.Method))
	material.WriteByte('\n')
	material.WriteString(normalizePath(req.Path))
	material.WriteByte('\n')
	material.WriteString(normalizeQuery(req.RawQuery))
	material.WriteByte('\n')
	material.WriteString(bodyHash(req.Method, req.Body))
	material.WriteByte('\n')
	material.WriteString(varyMaterial(req.Headers, req.VaryHeaders))

	sum := sha256.Sum256([]byte(material.String()))
	return hex.EncodeToString(sum[:])
}

func normalizePath(rawPath string) string {
	if rawPath == "" {
		return "/"
	}
	cleaned := path.Clean("/" + rawPath)
	if strings.HasSuffix(rawPath, "/") && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func normalizeQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
		sort.Strings(values[key])
	}
	sort.Strings(keys)

	normalized := url.Values{}
	for _, key := range keys {
		for _, value := range values[key] {
			normalized.Add(key, value)
		}
	}
	return normalized.Encode()
}

func bodyHash(method string, body []byte) string {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		sum := sha256.Sum256(body)
		return hex.EncodeToString(sum[:])
	default:
		return ""
	}
}

func varyMaterial(headers http.Header, varyHeaders []string) string {
	if len(varyHeaders) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(varyHeaders))
	for _, header := range varyHeaders {
		canonical := http.CanonicalHeaderKey(header)
		values := append([]string(nil), headers.Values(canonical)...)
		sort.Strings(values)
		normalized = append(normalized, canonical+":"+strings.Join(values, "\x00"))
	}
	sort.Strings(normalized)
	return strings.Join(normalized, "\n")
}
