package dashboard

import (
	"strings"

	"github.com/geofffranks/herdle/internal/config"
)

// knownGitHubHosts is the set of hosts treated as GitHub: the hosts gh is
// authenticated to (KnownHosts), always including github.com. Hosts are
// lowercased so matching against a remote URL's host is case-insensitive (DNS
// hostnames are; a "git@GitHub.com:o/r" remote must still resolve). hostFromURL
// also lowercases, so both sides of the comparison are canonical.
func (e Engine) knownGitHubHosts() map[string]bool {
	set := map[string]bool{"github.com": true}
	for _, h := range e.GH.KnownHosts() {
		if h != "" {
			set[strings.ToLower(h)] = true
		}
	}
	return set
}

// effectiveSlug applies the GitHub-host policy to a resolved project, returning
// the slug to pass to `gh -R` and whether PR features apply. A gh= override is
// trusted as-is (it may already be HOST/OWNER/REPO). A derived slug is host-gated
// and host-prefixed for GitHub Enterprise so `gh -R` targets the right server.
func effectiveSlug(r config.Resolved, known map[string]bool) (string, bool) {
	if r.SlugExplicit {
		return r.Slug, r.Slug != ""
	}
	if r.Slug == "" || r.RemoteHost == "" || !known[r.RemoteHost] {
		return "", false
	}
	if r.RemoteHost == "github.com" {
		return r.Slug, true
	}
	return r.RemoteHost + "/" + r.Slug, true
}
