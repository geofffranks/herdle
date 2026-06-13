package vcs_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
)

// gitStub writes an executable `git` stub in a fresh temp dir, points HERDLE_GIT
// at it, and returns the dir. The stub body is a /bin/sh script.
func gitStub(body string) string {
	dir := GinkgoT().TempDir()
	p := filepath.Join(dir, "git")
	Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_GIT", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_GIT") })
	return dir
}

var _ = Describe("GitRunner (env-override smoke + simple queries)", func() {
	var git vcs.GitRunner
	BeforeEach(func() { git = vcs.NewGitRunner() })

	It("honors HERDLE_GIT and parses the current branch", func() {
		dir := gitStub("#!/bin/sh\necho feature/x\n")
		br, err := git.CurrentBranch(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(br).To(Equal("feature/x"))
	})

	It("returns ErrNotARepo when rev-parse exits non-zero", func() {
		dir := gitStub("#!/bin/sh\nexit 128\n")
		_, err := git.RepoRoot(dir)
		Expect(err).To(MatchError(vcs.ErrNotARepo))
	})

	It("returns the repo root on success", func() {
		dir := gitStub("#!/bin/sh\necho /work/repo\n")
		root, err := git.RepoRoot(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(root).To(Equal("/work/repo"))
	})

	It("reads a remote url", func() {
		dir := gitStub("#!/bin/sh\necho git@github.com:o/r.git\n")
		url, err := git.RemoteURL(dir, "origin")
		Expect(err).NotTo(HaveOccurred())
		Expect(url).To(Equal("git@github.com:o/r.git"))
	})

	It("reports dirty when diff exits non-zero", func() {
		dir := gitStub("#!/bin/sh\nexit 1\n") // both diff calls non-zero
		dirty, err := git.IsDirty(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirty).To(BeTrue())
	})

	It("reports clean when diff exits zero", func() {
		dir := gitStub("#!/bin/sh\nexit 0\n")
		dirty, err := git.IsDirty(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirty).To(BeFalse())
	})

	It("reports dirty when only the index has staged changes", func() {
		// worktree diff (no --cached) exits 0; the --cached diff exits 1.
		dir := gitStub("#!/bin/sh\nfor a in \"$@\"; do [ \"$a\" = \"--cached\" ] && exit 1; done\nexit 0\n")
		dirty, err := git.IsDirty(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirty).To(BeTrue())
	})

	It("errors on fetch failure but succeeds on zero exit", func() {
		ok := gitStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGitRunner().Fetch(ok)).To(Succeed())
		bad := gitStub("#!/bin/sh\necho boom >&2\nexit 1\n")
		Expect(vcs.NewGitRunner().Fetch(bad)).To(HaveOccurred())
	})

	It("prunes a remote", func() {
		dir := gitStub("#!/bin/sh\nexit 0\n")
		Expect(git.PruneRemote(dir, "origin")).To(Succeed())
	})

	It("reads the remote HEAD branch, stripping the remote prefix", func() {
		dir := gitStub("#!/bin/sh\necho origin/main\n")
		head, err := git.RemoteHead(dir, "origin")
		Expect(err).NotTo(HaveOccurred())
		Expect(head).To(Equal("main"))
	})

	It("returns empty (no error) when no remote HEAD is set", func() {
		dir := gitStub("#!/bin/sh\nexit 1\n") // symbolic-ref fails -> caller falls back
		head, err := git.RemoteHead(dir, "origin")
		Expect(err).NotTo(HaveOccurred())
		Expect(head).To(Equal(""))
	})
})

var _ = Describe("GitRunner (divergence, refs, listings)", func() {
	var git vcs.GitRunner
	BeforeEach(func() { git = vcs.NewGitRunner() })

	It("parses left/right divergence counts", func() {
		dir := gitStub("#!/bin/sh\nprintf '2\\t5\\n'\n") // behind=2 ahead=5
		left, right, err := git.Divergence(dir, "@{upstream}", "HEAD")
		Expect(err).NotTo(HaveOccurred())
		Expect(left).To(Equal(2))
		Expect(right).To(Equal(5))
	})

	It("treats a non-zero rev-list (no upstream) as 0/0", func() {
		dir := gitStub("#!/bin/sh\nexit 128\n")
		left, right, err := git.Divergence(dir, "@{upstream}", "HEAD")
		Expect(err).NotTo(HaveOccurred())
		Expect(left).To(Equal(0))
		Expect(right).To(Equal(0))
	})

	It("maps show-ref exit 0/1 to exists true/false", func() {
		yes := gitStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGitRunner().LocalBranchExists(yes, "main")).To(BeTrue())
		no := gitStub("#!/bin/sh\nexit 1\n")
		Expect(vcs.NewGitRunner().RemoteBranchExists(no, "origin", "gone")).To(BeFalse())
	})

	It("parses local branches and the [gone] marker", func() {
		dir := gitStub("#!/bin/sh\nprintf 'main \\nfeature/x [ahead 1]\\nold [gone]\\n'\n")
		brs, err := git.LocalBranches(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(brs).To(Equal([]vcs.Branch{
			{Name: "main", UpstreamGone: false},
			{Name: "feature/x", UpstreamGone: false},
			{Name: "old", UpstreamGone: true},
		}))
	})

	It("lists remote branches with the remote prefix stripped and HEAD skipped", func() {
		dir := gitStub("#!/bin/sh\nprintf 'origin/main\\norigin/HEAD\\norigin/feature/y\\n'\n")
		brs, err := git.RemoteBranches(dir, "origin")
		Expect(err).NotTo(HaveOccurred())
		Expect(brs).To(Equal([]string{"main", "feature/y"}))
	})
})
