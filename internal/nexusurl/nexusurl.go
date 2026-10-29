// Package nexusurl extracts the Nexus Mods mod id from a mod-page URL.
package nexusurl

import (
	"net/url"
	"strconv"
	"strings"
)

// ID is the number after the "mods" path segment of a Nexus mod page URL
// (https://www.nexusmods.com/<game>/mods/<id>), or 0 when raw has none.
func ID(raw string) int {
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if !strings.EqualFold(parts[i], "mods") {
			continue
		}
		id, err := strconv.Atoi(parts[i+1])
		if err == nil && id > 0 {
			return id
		}
	}
	return 0
}
