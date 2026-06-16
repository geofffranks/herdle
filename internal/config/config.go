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
	// GH is the legacy GitHub-only owner/repo override. It still works and always
	// means GitHub; Slug is the forge-agnostic replacement.
	GH string `toml:"gh,omitempty"`
	// Slug is the forge-agnostic [GROUP/]OWNER/REPO override. The forge (GitHub vs
	// GitLab) is determined by the remote host, so this works for gitlab.com and
	// self-hosted GitLab as well as GitHub Enterprise.
	Slug        string `toml:"slug,omitempty"`
	Remote      string `toml:"remote,omitempty"`
	Base        string `toml:"base,omitempty"`        // trunk branch
	Integration string `toml:"integration,omitempty"` // personal integration branch
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
	// is no remote, the slug came only from a gh= override, or the URL is
	// unparseable). The dashboard uses it to decide whether a remote is a GitHub
	// (or GitHub Enterprise) host.
	RemoteHost string
	// SlugExplicit is true when Slug came from the project's gh= or slug= override
	// rather than from URL derivation; an explicit slug's value is trusted as-is.
	// A legacy gh= override additionally leaves RemoteHost empty (it is GitHub by
	// definition and needs no host probe); a neutral slug= override still resolves
	// RemoteHost so the dashboard can route it to the right forge.
	SlugExplicit bool
}
