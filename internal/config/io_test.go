package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
)

// tmpConfig points HERDLE_CONFIG at a fresh file in a temp dir and returns its path.
func tmpConfig() string {
	p := filepath.Join(GinkgoT().TempDir(), "herdle", "config.toml")
	os.Setenv("HERDLE_CONFIG", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_CONFIG") })
	return p
}

var _ = Describe("config IO", func() {
	It("loads a missing file as an empty config (no error)", func() {
		tmpConfig()
		c, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Projects).To(BeEmpty())
	})

	It("round-trips a sparse config, omitting unset fields", func() {
		path := tmpConfig()
		c := &config.Config{
			DefaultRemote: "upstream",
			Projects: []config.Project{
				{Path: "/work/a", GH: "o/a"},
				{Path: "/work/b", Base: "dev"},
			},
		}
		Expect(c.Save()).To(Succeed())

		raw, err := os.ReadFile(path) // #nosec G304 -- test reads the file it just wrote
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).To(ContainSubstring(`default_remote = "upstream"`))
		Expect(string(raw)).NotTo(ContainSubstring("default_base")) // unset -> omitted
		Expect(string(raw)).NotTo(ContainSubstring("integration"))  // unset -> omitted

		got, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(got.DefaultRemote).To(Equal("upstream"))
		Expect(got.Projects).To(Equal(c.Projects))
	})

	It("creates the parent directory on save", func() {
		path := tmpConfig() // parent dir does not exist yet
		c := &config.Config{Projects: []config.Project{{Path: "/x"}}}
		Expect(c.Save()).To(Succeed())
		_, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
	})

	It("errors and leaves no temp file when the rename fails", func() {
		dir := GinkgoT().TempDir()
		// A directory standing where the config file should be makes os.Rename
		// (file -> existing dir) fail, exercising the cleanup branch.
		path := filepath.Join(dir, "config.toml")
		Expect(os.Mkdir(path, 0o750)).To(Succeed())

		Expect((&config.Config{}).SaveTo(path)).NotTo(Succeed())

		entries, err := os.ReadDir(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1)) // only the colliding dir; the temp file was removed
		Expect(entries[0].Name()).To(Equal("config.toml"))
	})
})

var _ = Describe("baseDir-backed path helpers", func() {
	// For each spec we isolate all three env vars so the real HOME / XDG / CLAUDE
	// dirs don't bleed into assertions.
	var fakeHome string

	BeforeEach(func() {
		fakeHome = GinkgoT().TempDir()
		os.Setenv("HOME", fakeHome)
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("CLAUDE_CONFIG_DIR")
		os.Unsetenv("HERDLE_CONFIG")
		DeferCleanup(func() {
			os.Unsetenv("HOME")
			os.Unsetenv("XDG_CONFIG_HOME")
			os.Unsetenv("CLAUDE_CONFIG_DIR")
			os.Unsetenv("HERDLE_CONFIG")
		})
	})

	It("Path() falls back to $HOME/.config/herdle/config.toml", func() {
		p, err := config.Path()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(fakeHome, ".config", "herdle", "config.toml")))
	})

	It("Path() honours XDG_CONFIG_HOME", func() {
		xdg := GinkgoT().TempDir()
		os.Setenv("XDG_CONFIG_HOME", xdg)
		p, err := config.Path()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(xdg, "herdle", "config.toml")))
	})

	It("WipProjectsPath() falls back to $HOME/.config/wip/projects", func() {
		p, err := config.WipProjectsPath()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(fakeHome, ".config", "wip", "projects")))
	})

	It("WipProjectsPath() honours XDG_CONFIG_HOME", func() {
		xdg := GinkgoT().TempDir()
		os.Setenv("XDG_CONFIG_HOME", xdg)
		p, err := config.WipProjectsPath()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(xdg, "wip", "projects")))
	})

	It("ClaudeDir() falls back to $HOME/.claude", func() {
		p, err := config.ClaudeDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(fakeHome, ".claude")))
	})

	It("ClaudeDir() honours CLAUDE_CONFIG_DIR", func() {
		claudeDir := GinkgoT().TempDir()
		os.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
		p, err := config.ClaudeDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(claudeDir))
	})

	It("ClaudeProjectsDir() falls back to $HOME/.claude/projects", func() {
		p, err := config.ClaudeProjectsDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(fakeHome, ".claude", "projects")))
	})

	It("ClaudeProjectsDir() honours CLAUDE_CONFIG_DIR", func() {
		claudeDir := GinkgoT().TempDir()
		os.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
		p, err := config.ClaudeProjectsDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(claudeDir, "projects")))
	})
})
