package gale

import (
	"context"
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"

	httpkit "github.com/Rethunk-Tech/go-httpkit"
)

// codeID is the id Gale's server makes: six upper-case letters or digits.
var codeID = regexp.MustCompile(`^[A-Z0-9]{6}$`)

// legacyID is the id older Gale profiles have: a UUID in unpadded URL-safe base64, case-sensitive.
func legacyID(text string) bool {
	b, err := base64.RawURLEncoding.DecodeString(text)
	return err == nil && len(text) == 22 && len(b) == 16
}

// links are the prefixes Gale puts before a sync id: its "Copy link" address and its own deep link.
var links = []string{"https://gale.kesomannen.com/api/desktop/profile/sync/clone/", "gale://profile/sync/clone/"}

// CodeID is pasted text as the id Gale's server knows: a six-character code, upper-cased (people retype codes,
// and the server stores them upper-case), or an older profile's 22-character base64url UUID, unchanged. Gale's
// copied sync link and its gale:// deep link are read for the id at their end. ok is false for any other text.
func CodeID(text string) (id string, ok bool) {
	text = strings.TrimSpace(text)
	for _, prefix := range links {
		if rest, found := strings.CutPrefix(text, prefix); found {
			text = strings.TrimSuffix(rest, "/")
			break
		}
	}
	if legacyID(text) {
		return text, true
	}
	if up := strings.ToUpper(text); codeID.MatchString(up) {
		return up, true
	}
	return "", false
}

// Client reads sync codes from Gale's server.
type Client struct {
	HTTP      *http.Client // nil means http.DefaultClient; redirects are followed (to Thunderstore)
	BaseURL   string       // "" means https://gale.kesomannen.com/api
	UserAgent string
	MaxBytes  int64 // 0 means 128 MiB
}

// FetchCode downloads the .r2z body for id (GET /profile/<id>). The server answers with a redirect to the
// profile on Thunderstore and needs no account. A non-200 answer is a *httpkit.StatusError (httpkit.ErrNotFound,
// httpkit.ErrRateLimited match it); a body over MaxBytes is safefs.ErrTooLarge. The deadline is the caller's
// context.
func (c Client) FetchCode(ctx context.Context, id string) ([]byte, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://gale.kesomannen.com/api"
	}
	limit := c.MaxBytes
	if limit == 0 {
		limit = 128 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/profile/"+id, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	return httpkit.DoCapped(c.HTTP, req, limit, nil)
}
