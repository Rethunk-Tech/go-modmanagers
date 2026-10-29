package r2modman

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	httpkit "github.com/Rethunk-Tech/go-httpkit"
)

// CodePrefix starts a pasted profile code.
const CodePrefix = "#r2modman"

// importLink is the deep link Gale's exported code arrives in; the key after it is Thunderstore's.
const importLink = "gale://profile/import/"

var codeKey = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsCode reports text that, trimmed, starts with CodePrefix.
func IsCode(text string) bool { return strings.HasPrefix(strings.TrimSpace(text), CodePrefix) }

// CodeKey is the key Thunderstore stores a code under: pasted text that, trimmed, is a bare UUID or a Gale
// "gale://profile/import/<uuid>" deep link (a trailing slash allowed). ok is false for any other text.
func CodeKey(text string) (key string, ok bool) {
	text = strings.TrimSpace(text)
	if k, found := strings.CutPrefix(text, importLink); found {
		text = strings.TrimSuffix(k, "/")
	}
	return text, codeKey.MatchString(text)
}

// IsCodeKey reports whether CodeKey accepts text.
func IsCodeKey(text string) bool {
	_, ok := CodeKey(text)
	return ok
}

// DecodeCode is the zip a pasted code carries: CodePrefix, then base64 with any whitespace.
func DecodeCode(text string) ([]byte, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(text), CodePrefix)
	if !ok {
		return nil, errors.New("not an r2modman profile code")
	}
	b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(body), ""))
	if err != nil {
		return nil, fmt.Errorf("profile code is not base64: %w", err)
	}
	return b, nil
}

// Client reads codes from Thunderstore.
type Client struct {
	HTTP      *http.Client // nil means http.DefaultClient
	BaseURL   string       // "" means https://thunderstore.io
	UserAgent string       // "" leaves the client's own
	MaxBytes  int64        // 0 means 64 MiB
}

// FetchCode reads the code stored under key (GET /api/experimental/legacyprofile/get/<key>/). A non-200 answer is
// a *httpkit.StatusError (httpkit.ErrNotFound and httpkit.ErrRateLimited match it); a body over MaxBytes is
// safefs.ErrTooLarge. The deadline is the caller's context.
func (c Client) FetchCode(ctx context.Context, key string) (string, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://thunderstore.io"
	}
	limit := c.MaxBytes
	if limit == 0 {
		limit = 64 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/experimental/legacyprofile/get/"+key+"/", nil)
	if err != nil {
		return "", err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	b, err := httpkit.DoCapped(c.HTTP, req, limit, nil)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
