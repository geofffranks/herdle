package initcmd_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/initcmd"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

var _ = Describe("SeedConfig", func() {
	It("seeds from wip migrate + claude discover when config is absent", func() {
		tmp := GinkgoT().TempDir()
		configPath := filepath.Join(tmp, "config.toml")

		wipPath := filepath.Join(tmp, "wip-projects")
		Expect(os.WriteFile(wipPath, []byte("/work/a gh=o/a\n/work/b\n"), 0o600)).To(Succeed())

		projects := filepath.Join(tmp, "claude-projects")
		entry := filepath.Join(projects, "-work-c")
		Expect(os.MkdirAll(entry, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(entry, "s.jsonl"), []byte(`{"cwd":"/work/c"}`+"\n"), 0o600)).To(Succeed())

		git := &vcsfakes.FakeGitRunner{}
		git.RepoRootStub = func(p string) (string, error) { return p, nil }

		n, ran, err := initcmd.SeedConfig(configPath, wipPath, projects, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(ran).To(BeTrue())
		Expect(n).To(Equal(3))
		Expect(configPath).To(BeAnExistingFile())

		got, err := config.LoadFrom(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Projects).To(HaveLen(3))
	})

	It("is a no-op when config already exists (gate closed before any work)", func() {
		tmp := GinkgoT().TempDir()
		configPath := filepath.Join(tmp, "config.toml")
		Expect(os.WriteFile(configPath, []byte(""), 0o600)).To(Succeed())

		git := &vcsfakes.FakeGitRunner{}
		n, ran, err := initcmd.SeedConfig(configPath, "/absent/wip", "/absent/projects", git)
		Expect(err).NotTo(HaveOccurred())
		Expect(ran).To(BeFalse())
		Expect(n).To(Equal(0))
		Expect(git.RepoRootCallCount()).To(Equal(0))
	})

	It("writes an empty config (closing the gate) when nothing is discovered", func() {
		tmp := GinkgoT().TempDir()
		configPath := filepath.Join(tmp, "config.toml")
		git := &vcsfakes.FakeGitRunner{}

		n, ran, err := initcmd.SeedConfig(configPath, "/absent/wip", "/absent/projects", git)
		Expect(err).NotTo(HaveOccurred())
		Expect(ran).To(BeTrue())
		Expect(n).To(Equal(0))
		Expect(configPath).To(BeAnExistingFile())
	})

	It("returns the stat error (gate undecided) when configPath is inaccessible, without seeding", func() {
		tmp := GinkgoT().TempDir()
		// A regular file stands where a directory is expected, so os.Stat on
		// configPath fails with ENOTDIR — a real error that is NOT fs.ErrNotExist.
		notDir := filepath.Join(tmp, "afile")
		Expect(os.WriteFile(notDir, []byte("x"), 0o600)).To(Succeed())
		configPath := filepath.Join(notDir, "config.toml")

		git := &vcsfakes.FakeGitRunner{}
		n, ran, err := initcmd.SeedConfig(configPath, "/absent/wip", "/absent/projects", git)
		Expect(err).To(HaveOccurred())               // surfaced, not swallowed as "absent"
		Expect(ran).To(BeFalse())                    // gate did not open
		Expect(n).To(Equal(0))                       // nothing seeded
		Expect(git.RepoRootCallCount()).To(Equal(0)) // bailed before any discovery work
	})

	It("keeps the wip gh= slug when the same path is also discovered (migrate-first)", func() {
		tmp := GinkgoT().TempDir()
		configPath := filepath.Join(tmp, "config.toml")

		wipPath := filepath.Join(tmp, "wip-projects")
		Expect(os.WriteFile(wipPath, []byte("/work/dup gh=o/dup\n"), 0o600)).To(Succeed())

		projects := filepath.Join(tmp, "claude-projects")
		entry := filepath.Join(projects, "-work-dup")
		Expect(os.MkdirAll(entry, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(entry, "s.jsonl"), []byte(`{"cwd":"/work/dup"}`+"\n"), 0o600)).To(Succeed())

		git := &vcsfakes.FakeGitRunner{}
		git.RepoRootStub = func(p string) (string, error) { return p, nil }

		n, _, err := initcmd.SeedConfig(configPath, wipPath, projects, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(1)) // deduped by path

		got, err := config.LoadFrom(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Projects).To(HaveLen(1))
		Expect(got.Projects[0].GH).To(Equal("o/dup")) // wip slug survived
	})
})
