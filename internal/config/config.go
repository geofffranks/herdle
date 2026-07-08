// Package config is herdle's project store: a sparse TOML file managed by the
// `herdle project` CLI. Only explicitly-set fields serialize (omitempty); unset
// fields are filled by Config.Resolve at read time against the live repo, so the
// file stays pure user-intent and never bakes in autodetected values.
package config

// Config is the on-disk store.
type Config struct {
	DefaultRemote string    `toml:"default_remote,omitempty"`
	DefaultBase   string    `toml:"default_base,omitempty"`
	Projects      []Project `toml:"project,omitempty"`
}

// Project is one tracked repo. Every field but Path is optional; unset fields are
// filled by Resolve.
type Project struct {
	Path string `toml:"path"`
	// Slug is the forge-agnostic [GROUP/]OWNER/REPO override. The forge (GitHub vs
	// GitLab) is determined by the remote host, so this works for gitlab.com and
	// self-hosted GitLab as well as GitHub Enterprise.
	Slug        string `toml:"slug,omitempty"`
	Remote      string `toml:"remote,omitempty"`
	Base        string `toml:"base,omitempty"`        // trunk branch
	Integration string `toml:"integration,omitempty"` // personal integration branch
	// GH is the legacy GitHub-only slug key (`gh = "owner/repo"`), written by the
	// old `--gh` flow before the move to the forge-agnostic Slug. It is read on load
	// and folded into Slug (see foldLegacyGH); never written back, so an upgraded
	// config re-saves as `slug =`. Kept only so a stale `gh =` does not silently lose
	// PR correlation. Always GitHub (the legacy key predates GitLab support).
	GH string `toml:"gh,omitempty"`
	// Issues overrides source-of-truth issue tracking: nil autodetects (source ⟺ no
	// upstream remote), true forces on, false forces off. Hand-set in config.toml.
	Issues *bool `toml:"issues,omitempty"`
}

// foldLegacyGH migrates a legacy `gh =` slug into the forge-agnostic Slug, in
// place, and clears GH so it does not round-trip back to disk. An explicit modern
// `slug =` always wins; gh= only fills an otherwise-empty Slug.
func (p *Project) foldLegacyGH() {
	if p.GH == "" {
		return
	}
	if p.Slug == "" {
		p.Slug = p.GH
	}
	p.GH = ""
}

// Resolved is the fully-filled view a consumer (dashboard, project list) uses.
type Resolved struct {
	Path        string
	Name        string
	Remote      string
	Base        string
	Integration string
	Slug        string
	// RemoteHost is the host parsed from the resolved remote's URL ("" when there
	// is no remote or the URL is unparseable), with any port stripped. The dashboard
	// uses it to route the project to the right forge (GitHub/GitHub Enterprise vs
	// GitLab/self-hosted); forge KnownHosts are likewise port-free, so routing
	// matches on the bare host.
	RemoteHost string
	// RemoteHostPort is RemoteHost with its original port retained (e.g.
	// "gitlab.internal:8929"); equal to RemoteHost when the URL had no port. Routing
	// uses the port-free RemoteHost, but building a glab `-R https://HOST/slug` URL
	// for self-hosted GitLab must carry the port or glab hits the default 443/80.
	RemoteHostPort string
	// SlugExplicit is true when Slug came from the project's slug= override rather
	// than from URL derivation; an explicit slug's value is trusted as-is. Forge is
	// still chosen by RemoteHost, so an explicit slug with no resolvable host
	// degrades rather than guessing a forge.
	SlugExplicit bool
	// TrackIssues is whether forge issues are listed for this repo. True for a
	// source-of-truth repo (no upstream remote) unless Project.Issues overrides.
	TrackIssues bool
}
