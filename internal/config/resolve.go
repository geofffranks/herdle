package config

import (
	"path/filepath"
	"strings"

	"github.com/geofffranks/herdle/internal/vcs"
)

// Resolve fills unset project fields against the live repo: explicit value ->
// global default -> autodetect. Git errors are treated as "field unavailable" and
// the chain falls through, so Resolve does not fail on a missing remote (the
// error return is reserved for future hard failures).
func (c *Config) Resolve(p Project, git vcs.GitRunner) (Resolved, error) {
	r := Resolved{Path: p.Path, Name: filepath.Base(p.Path)}

	// remote: explicit -> default -> origin -> upstream -> ""
	// (origin-first: branches live on your push remote; for a fork, the PR slug
	// comes from the slug= override.) autodetectedURL caches the URL fetched here so
	// the slug/host branch can reuse it without a second RemoteURL call.
	var autodetectedURL string
	r.Remote = p.Remote
	if r.Remote == "" {
		r.Remote = c.DefaultRemote
	}
	if r.Remote == "" {
		if url, err := git.RemoteURL(p.Path, "origin"); err == nil {
			r.Remote = "origin"
			autodetectedURL = url
		} else if url, err := git.RemoteURL(p.Path, "upstream"); err == nil {
			r.Remote = "upstream"
			autodetectedURL = url
		}
	}

	// base: explicit -> default -> RemoteHead -> main -> master -> "main"
	switch {
	case p.Base != "":
		r.Base = p.Base
	case c.DefaultBase != "":
		r.Base = c.DefaultBase
	case r.Remote == "":
		r.Base = "main"
	default:
		if head, _ := git.RemoteHead(p.Path, r.Remote); head != "" {
			r.Base = head
		} else if ok, _ := git.RemoteBranchExists(p.Path, r.Remote, "main"); ok {
			r.Base = "main"
		} else if ok, _ := git.RemoteBranchExists(p.Path, r.Remote, "master"); ok {
			r.Base = "master"
		} else {
			r.Base = "main"
		}
	}

	// integration: explicit only (personal branch; never autodetected)
	r.Integration = p.Integration

	// slug: explicit slug= (forge by host) -> derived from the remote URL -> "".
	if p.Slug != "" {
		r.Slug = p.Slug
		r.SlugExplicit = true
	}

	// Host detection (and slug derivation when no override is set) runs whenever a
	// remote exists. The host is what routes the project to GitHub vs GitLab, so it
	// is always probed — even for an explicit slug= override, whose value is trusted
	// but whose forge still comes from the remote host.
	if r.Remote != "" {
		// Reuse the URL cached during autodetection if available; otherwise fetch.
		url := autodetectedURL
		if url == "" {
			url, _ = git.RemoteURL(p.Path, r.Remote)
		}
		if url != "" {
			r.RemoteHostPort = authorityFromURL(url)
			r.RemoteHost = stripPort(r.RemoteHostPort)
			if r.Slug == "" {
				r.Slug = slugFromURL(url)
			}
		}
	}

	// TrackIssues: forge issues are listed only for source-of-truth repos. The fork
	// convention names the canonical repo "upstream", so its presence is the fork
	// signal — a local check, no forge round-trip. An explicit issues= override wins
	// and short-circuits the upstream probe, so a project pinned with issues= pays no
	// extra git call.
	if p.Issues != nil {
		r.TrackIssues = *p.Issues
	} else {
		hasUpstream := r.Remote == "upstream" // already resolved to it => it exists
		if !hasUpstream {
			if _, err := git.RemoteURL(p.Path, "upstream"); err == nil {
				hasUpstream = true
			}
		}
		r.TrackIssues = !hasUpstream
	}

	return r, nil
}

// authorityFromURL extracts the host authority (host with any port retained) from
// a git remote URL, mirroring slugFromURL's scheme handling:
// "git@host:owner/repo(.git)" -> "host" (scp-like syntax carries no port);
// "scheme://[user@]host[:port]/owner/repo(.git)" -> "host[:port]"; anything else
// -> "". The result is lowercased. Callers that route by host strip the port via
// stripPort; callers that rebuild a forge URL (self-hosted GitLab) keep it.
func authorityFromURL(url string) string {
	s := strings.TrimSpace(url)
	switch {
	case strings.HasPrefix(s, "git@"):
		// scp-like "git@host:path": the colon separates host from path, not a port,
		// so there is no port to retain here.
		s = strings.TrimPrefix(s, "git@")
		if i := strings.IndexByte(s, ':'); i > 0 {
			return strings.ToLower(s[:i])
		}
		return ""
	case strings.Contains(s, "://"):
		s = s[strings.Index(s, "://")+3:]
		if at := strings.IndexByte(s, '@'); at >= 0 {
			s = s[at+1:]
		}
		authority := s
		if i := strings.IndexByte(s, '/'); i >= 0 {
			authority = s[:i]
		}
		return strings.ToLower(authority)
	}
	return ""
}

// stripPort removes a trailing :port from a URL authority, leaving a bracketed
// IPv6 literal ("[::1]") intact (its inner colons are not a port separator).
func stripPort(host string) string {
	if strings.HasPrefix(host, "[") {
		if i := strings.IndexByte(host, ']'); i >= 0 {
			return host[:i+1]
		}
		return host
	}
	if j := strings.LastIndexByte(host, ':'); j >= 0 {
		return host[:j]
	}
	return host
}

// slugFromURL extracts the project path from a git remote URL: strip the
// scheme+host (git@host: or scheme://host/) and a trailing .git. The result is
// owner/repo for GitHub, but GitLab allows arbitrarily nested groups
// (group/subgroup/.../project), so any path of two or more non-empty segments is
// accepted. Returns "" when the result is not path-shaped.
//
// Known, accepted limitation: this validates path SHAPE, not forge-specific
// arity. A malformed 3+-segment github.com remote (e.g. ".../owner/repo/extra")
// is therefore accepted here and only fails later, at the forge call, surfacing
// as a "?" cell rather than degrading to "-". We do NOT guard it: forge identity
// isn't known at this layer (a 3-segment path is a valid GitLab subgroup but an
// invalid GitHub repo, and a GitHub Enterprise host is indistinguishable from a
// self-hosted GitLab one here), so the only guard possible would special-case the
// literal "github.com" — arbitrary, and for input that does not occur from a
// real `git remote get-url`. Only auto-derived slugs reach this function;
// explicit gh=/slug= overrides (which legitimately can be host-qualified and
// 3-segment) bypass it.
func slugFromURL(url string) string {
	s := strings.TrimSpace(url)
	switch {
	case strings.HasPrefix(s, "git@"):
		if i := strings.IndexByte(s, ':'); i >= 0 {
			s = s[i+1:]
		}
	case strings.Contains(s, "://"):
		s = s[strings.Index(s, "://")+3:]
		i := strings.IndexByte(s, '/')
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	// Require at least owner/repo (two segments). GitLab nested groups push this
	// deeper (group/subgroup/.../project); reject only malformed paths — a leading
	// or trailing slash, or any empty segment (e.g. "a//b").
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return ""
	}
	for _, p := range parts {
		if p == "" {
			return ""
		}
	}
	return s
}
