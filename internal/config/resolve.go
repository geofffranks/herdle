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

	// slug: explicit gh -> derived from the remote URL -> ""
	r.Slug = p.GH
	r.SlugExplicit = p.GH != ""
	if r.Slug == "" && r.Remote != "" {
		// Reuse the URL cached during autodetection if available; otherwise fetch.
		url := autodetectedURL
		if url == "" {
			url, _ = git.RemoteURL(p.Path, r.Remote)
		}
		if url != "" {
			r.Slug = slugFromURL(url)
			r.RemoteHost = hostFromURL(url)
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

// slugFromURL extracts owner/repo from a git remote URL, mirroring wip's
// slug_from_url: strip the scheme+host (git@host: or scheme://host/) and a
// trailing .git. Returns "" if the result is not owner/repo-shaped.
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
	if strings.Count(s, "/") != 1 || strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return ""
	}
	return s
}
