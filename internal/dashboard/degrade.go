package dashboard

import (
	"strings"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// forgeClient is the subset of a forge runner the engine needs to fetch and gate
// PR/MR data. Both vcs.GHRunner and vcs.GLRunner satisfy it, so the gather logic
// is forge-neutral once a project has been routed to one of them.
type forgeClient interface {
	PRList(slug, state string) ([]vcs.PR, error)
	Available() bool
}

// hostSet lowercases and indexes a forge's known hosts, always including the
// forge's canonical public host. Lowercasing keeps matching against a remote
// URL's host case-insensitive (hostFromURL also lowercases).
func hostSet(canonical string, hosts []string) map[string]bool {
	set := map[string]bool{canonical: true}
	for _, h := range hosts {
		if h != "" {
			set[strings.ToLower(h)] = true
		}
	}
	return set
}

// forgeRouting is the resolved host->forge policy for one dashboard run: the
// GitHub host set (always present, seeded with github.com) and the GitLab host
// set (only populated when a glab runner is wired, seeded with gitlab.com). Each
// forge's KnownHosts is queried once per run.
type forgeRouting struct {
	githubHosts map[string]bool
	gitlabHosts map[string]bool // nil when no GL runner is configured
}

// routing builds the host->forge policy from the wired forges' known hosts.
func (e Engine) routing() forgeRouting {
	r := forgeRouting{githubHosts: hostSet("github.com", e.GH.KnownHosts())}
	if e.GL != nil {
		r.gitlabHosts = hostSet("gitlab.com", e.GL.KnownHosts())
	}
	return r
}

// kind classifies a host as "github", "gitlab", or "" (belongs to no wired
// forge). GitHub wins a tie (a host configured on both), which is vanishingly
// unlikely and preserves legacy behavior.
func (rt forgeRouting) kind(host string) string {
	if host == "" {
		return ""
	}
	if rt.githubHosts[host] {
		return "github"
	}
	if rt.gitlabHosts != nil && rt.gitlabHosts[host] {
		return "gitlab"
	}
	return ""
}

// forgeCLI maps a forge kind to the CLI binary name herdle drives for it, used
// in user-facing degradation notes ("glab not found", "gh unavailable").
func forgeCLI(kind string) string {
	if kind == "gitlab" {
		return "glab"
	}
	return "gh"
}

// clientFor returns the runner for a forge kind. GitLab is only returned when a
// glab runner is wired; everything else falls back to the (always-present) GH
// runner, which keeps a legacy gh= override pointed at GitHub.
func (e Engine) clientFor(kind string) forgeClient {
	if kind == "gitlab" && e.GL != nil {
		return e.GL
	}
	return e.GH
}

// selectForge applies the host policy to a resolved project, returning the forge
// client, the slug to pass it, the forge kind, and whether PR/MR features apply.
//
// Forge selection is by RemoteHost. A slug — whether explicit (gh=/slug=) or
// derived from the remote URL — is host-qualified for a known, non-canonical host
// so the CLI targets the right server: host-prefixed for gh (HOST/OWNER/REPO),
// and a full https URL for glab (whose -R also accepts group/subgroup paths, so a
// bare host prefix would be ambiguous). github.com / gitlab.com need no prefix.
//
// A legacy gh= override, or any explicit slug whose host is unknown (e.g. set
// without a recognizable remote), is GitHub by default and trusted as-is — it may
// already be HOST/OWNER/REPO, so it is never rewritten.
func (e Engine) selectForge(r config.Resolved, rt forgeRouting) (forgeClient, string, string, bool) {
	var kind string
	if r.SlugExplicit {
		if r.Slug == "" {
			return nil, "", "", false
		}
		kind = rt.kind(r.RemoteHost)
		if kind == "" {
			// legacy gh= / unknown host: GitHub, slug trusted exactly as given.
			return e.clientFor("github"), r.Slug, "github", true
		}
	} else {
		if r.Slug == "" || r.RemoteHost == "" {
			return nil, "", "", false
		}
		kind = rt.kind(r.RemoteHost)
		if kind == "" {
			return nil, "", "", false
		}
	}

	// kind is github or gitlab and RemoteHost is a known host of that forge:
	// qualify the slug for a non-canonical (self-hosted / Enterprise) host.
	slug := r.Slug
	switch kind {
	case "github":
		if r.RemoteHost != "github.com" {
			slug = r.RemoteHost + "/" + slug
		}
	case "gitlab":
		if r.RemoteHost != "gitlab.com" {
			slug = "https://" + r.RemoteHost + "/" + slug
		}
	}
	return e.clientFor(kind), slug, kind, true
}
