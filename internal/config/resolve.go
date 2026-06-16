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
	// (origin-first: branches live on your push remote; the PR slug for forks
	// comes from the gh= override.) autodetectedURL caches the URL fetched here so
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

	// slug: explicit gh= (legacy, GitHub) -> explicit slug= (forge by host) ->
	// derived from the remote URL -> "".
	switch {
	case p.GH != "":
		r.Slug = p.GH
		r.SlugExplicit = true
	case p.Slug != "":
		r.Slug = p.Slug
		r.SlugExplicit = true
	}

	// Host detection (and slug derivation when no override is set) runs whenever a
	// remote exists, EXCEPT for a legacy gh= override — that is GitHub by
	// definition and needs no host probe, so it leaves RemoteHost empty and skips
	// the extra RemoteURL call. A neutral slug= override still probes the host so
	// the dashboard can route it to GitHub vs GitLab.
	if p.GH == "" && r.Remote != "" {
		// Reuse the URL cached during autodetection if available; otherwise fetch.
		url := autodetectedURL
		if url == "" {
			url, _ = git.RemoteURL(p.Path, r.Remote)
		}
		if url != "" {
			r.RemoteHost = hostFromURL(url)
			if r.Slug == "" {
				r.Slug = slugFromURL(url)
			}
		}
	}

	return r, nil
}

// hostFromURL extracts the host from a git remote URL, mirroring slugFromURL's
// scheme handling: "git@host:owner/repo(.git)" and
// "scheme://[user@]host[:port]/owner/repo(.git)" -> "host" (port stripped);
// anything else -> "".
func hostFromURL(url string) string {
	s := strings.TrimSpace(url)
	switch {
	case strings.HasPrefix(s, "git@"):
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
		host := s
		if i := strings.IndexByte(s, '/'); i >= 0 {
			host = s[:i]
		}
		return strings.ToLower(stripPort(host))
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
