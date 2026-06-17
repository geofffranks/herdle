package config_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

// projectsTree builds a fake ~/.claude/projects dir. Each entry maps an encoded
// dir name to the cwd written into a single transcript line ("" = no transcript).
func projectsTree(entries map[string]string) string {
	root := GinkgoT().TempDir()
	for name, cwd := range entries {
		dir := filepath.Join(root, name)
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		if cwd == "" {
			continue
		}
		line := `{"type":"summary","cwd":"` + cwd + `"}` + "\n"
		Expect(os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(line), 0o600)).To(Succeed())
	}
	return root
}

var _ = Describe("DiscoverClaudeProjects", func() {
	It("resolves each transcript cwd to a repo root, deduped, skipping non-repos", func() {
		root := projectsTree(map[string]string{
			"-Users-me-work-herdle":          "/Users/me/work/herdle",
			"-Users-me-work-herdle-internal": "/Users/me/work/herdle/internal", // subdir -> same root
			"-Users-me":                      "/Users/me",                      // home -> not a repo
			"-Users-me-work-empty":           "",                               // no transcript
		})
		git := &vcsfakes.FakeGitRunner{}
		git.RepoRootStub = func(path string) (string, error) {
			switch path {
			case "/Users/me/work/herdle", "/Users/me/work/herdle/internal":
				return "/Users/me/work/herdle", nil
			default:
				return "", vcs.ErrNotARepo
			}
		}
		roots, err := config.DiscoverClaudeProjects(root, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(roots).To(ConsistOf("/Users/me/work/herdle")) // deduped, non-repos dropped
	})

	It("returns nil (no error) when the projects dir is absent", func() {
		git := &vcsfakes.FakeGitRunner{}
		roots, err := config.DiscoverClaudeProjects(filepath.Join(GinkgoT().TempDir(), "absent"), git)
		Expect(err).NotTo(HaveOccurred())
		Expect(roots).To(BeEmpty())
	})

	It("falls back to a second transcript when the first has no cwd", func() {
		root := GinkgoT().TempDir()
		dir := filepath.Join(root, "-Users-me-work-herdle")
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		// a.jsonl — lexicographically first, no cwd line
		Expect(os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(`{"type":"summary"}`+"\n"), 0o600)).To(Succeed())
		// b.jsonl — has a valid cwd
		Expect(os.WriteFile(filepath.Join(dir, "b.jsonl"), []byte(`{"type":"summary","cwd":"/Users/me/work/herdle"}`+"\n"), 0o600)).To(Succeed())

		git := &vcsfakes.FakeGitRunner{}
		git.RepoRootStub = func(path string) (string, error) {
			if path == "/Users/me/work/herdle" {
				return "/Users/me/work/herdle", nil
			}
			return "", vcs.ErrNotARepo
		}
		roots, err := config.DiscoverClaudeProjects(root, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(roots).To(ConsistOf("/Users/me/work/herdle"))
	})

	It("feeds Add to seed a config without duplicating an existing entry", func() {
		root := projectsTree(map[string]string{"-Users-me-work-herdle": "/Users/me/work/herdle"})
		git := &vcsfakes.FakeGitRunner{}
		git.RepoRootStub = func(path string) (string, error) { return path, nil }
		roots, err := config.DiscoverClaudeProjects(root, git)
		Expect(err).NotTo(HaveOccurred())

		c := &config.Config{Projects: []config.Project{{Path: "/Users/me/work/herdle", Slug: "o/h"}}}
		for _, p := range roots {
			_ = c.Add(config.Project{Path: p})
		}
		Expect(c.Projects).To(HaveLen(1)) // already present -> not re-added
		Expect(c.Projects[0].Slug).To(Equal("o/h"))
		_ = errors.New // keep errors import tidy if unused elsewhere
	})
})
