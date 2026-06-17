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
// URL's host case-insensitive (authorityFromURL also lowercases).
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

// forgeAvailability probes each wired forge's CLI once, keyed by forge kind
// ("github"/"gitlab"). Availability is a process-wide fact (binary on PATH), so
// computing it once per run lets the per-project fan-out look it up by kind
// instead of re-running Available() for every project.
func (e Engine) forgeAvailability() map[string]bool {
	avail := map[string]bool{"github": e.GH.Available()}
	if e.GL != nil {
		avail["gitlab"] = e.GL.Available()
	}
	return avail
}

// clientFor returns the runner for a forge kind. GitLab is only returned when a
// glab runner is wired; everything else uses the (always-present) GH runner.
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
// An explicit slug we can't route — its host belongs to no wired/known forge, or
// it has no resolvable host at all — degrades to "-" rather than guessing a forge:
// routing, say, a self-hosted GitLab slug over to gh would only produce a phantom
// "?".
func (e Engine) selectForge(r config.Resolved, rt forgeRouting) (forgeClient, string, string, bool) {
	var kind string
	if r.SlugExplicit {
		if r.Slug == "" {
			return nil, "", "", false
		}
		kind = rt.kind(r.RemoteHost)
		if kind == "" {
			// An explicit slug= we can't route: either its host belongs to no
			// wired/known forge (e.g. a self-hosted GitLab glab isn't authed to), or
			// no host resolved at all. We can't know the forge, so degrade gracefully
			// to "-" rather than guess GitHub and emit a phantom "?".
			return nil, "", "", false
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
			// Use the port-qualified authority: a self-hosted GitLab on a non-default
			// port (e.g. gitlab.internal:8929) must keep its port or glab's -R URL hits
			// 443/80. RemoteHostPort == RemoteHost when the URL had no port.
			host := r.RemoteHostPort
			if host == "" {
				host = r.RemoteHost
			}
			slug = "https://" + host + "/" + slug
		}
	}
	return e.clientFor(kind), slug, kind, true
}
