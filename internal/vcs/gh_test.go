package vcs_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
)

// ghStub writes an executable `gh` stub, points HERDLE_GH at it, returns its dir
// (handy for retry-counter state).
func ghStub(body string) string {
	dir := GinkgoT().TempDir()
	p := filepath.Join(dir, "gh")
	Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_GH", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_GH") })
	return dir
}

var _ = Describe("GHRunner.PRList", func() {
	var gh vcs.GHRunner
	BeforeEach(func() { gh = vcs.NewGHRunner() })

	It("parses a JSON array of PRs", func() {
		ghStub(`#!/bin/sh
echo '[{"number":12,"state":"OPEN","headRefName":"feat/x","title":"Add x"}]'
`)
		prs, err := gh.PRList("o/r", "open")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(Equal([]vcs.PR{{Number: 12, State: "OPEN", HeadRefName: "feat/x", Title: "Add x"}}))
	})

	It("treats an empty JSON array as zero PRs (no error)", func() {
		ghStub("#!/bin/sh\necho '[]'\n")
		prs, err := gh.PRList("o/r", "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(BeEmpty())
	})

	It("retries once then succeeds", func() {
		dir := ghStub(`#!/bin/sh
c="$0.n"; n=$(cat "$c" 2>/dev/null || echo 0); echo $((n+1)) > "$c"
if [ "$n" = "0" ]; then echo "transient" >&2; exit 1; fi
echo '[]'
`)
		_, err := gh.PRList("o/r", "open")
		Expect(err).NotTo(HaveOccurred())
		data, readErr := os.ReadFile(filepath.Join(dir, "gh.n"))
		Expect(readErr).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(string(data))).To(Equal("2")) // exactly 2 attempts, not more
	})

	It("returns an error (not empty) when gh fails both attempts", func() {
		ghStub("#!/bin/sh\necho boom >&2\nexit 1\n")
		_, err := gh.PRList("o/r", "open")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the gh binary is unavailable", func() {
		os.Setenv("HERDLE_GH", "/nonexistent/gh-binary")
		DeferCleanup(func() { os.Unsetenv("HERDLE_GH") })
		_, err := gh.PRList("o/r", "open")
		Expect(err).To(HaveOccurred()) // graceful degradation: unavailable gh -> error, never empty
	})

	It("returns an error when output is not a JSON array", func() {
		ghStub("#!/bin/sh\necho 'not json'\n")
		_, err := gh.PRList("o/r", "open")
		Expect(err).To(HaveOccurred())
	})

	It("parses the merge/review/draft/check fields incl. a mixed rollup", func() {
		ghStub(`#!/bin/sh
echo '[{"number":12,"state":"OPEN","headRefName":"feat/x","title":"Add x","mergeable":"CONFLICTING","reviewDecision":"CHANGES_REQUESTED","isDraft":true,"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"build"},{"__typename":"StatusContext","state":"SUCCESS","context":"ci/lint"}]}]'
`)
		prs, err := gh.PRList("o/r", "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(HaveLen(1))
		Expect(prs[0].Mergeable).To(Equal("CONFLICTING"))
		Expect(prs[0].ReviewDecision).To(Equal("CHANGES_REQUESTED"))
		Expect(prs[0].IsDraft).To(BeTrue())
		Expect(prs[0].StatusCheckRollup).To(HaveLen(2))
		Expect(prs[0].StatusCheckRollup[0].Conclusion).To(Equal("FAILURE"))
		Expect(prs[0].StatusCheckRollup[1].State).To(Equal("SUCCESS"))
	})

	It("requests the merge/review/check fields from gh", func() {
		dir := ghStub(`#!/bin/sh
echo "$@" > "$0.args"
echo '[]'
`)
		_, err := gh.PRList("o/r", "open")
		Expect(err).NotTo(HaveOccurred())
		args, readErr := os.ReadFile(filepath.Join(dir, "gh.args"))
		Expect(readErr).NotTo(HaveOccurred())
		for _, f := range []string{"mergeable", "reviewDecision", "isDraft", "statusCheckRollup"} {
			Expect(string(args)).To(ContainSubstring(f))
		}
	})
})

var _ = Describe("GHRunner.Available", func() {
	It("is true when HERDLE_GH points at an existing binary", func() {
		ghStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGHRunner().Available()).To(BeTrue())
	})

	It("is false when HERDLE_GH points at a missing path", func() {
		os.Setenv("HERDLE_GH", filepath.Join(GinkgoT().TempDir(), "nope-gh"))
		DeferCleanup(func() { os.Unsetenv("HERDLE_GH") })
		Expect(vcs.NewGHRunner().Available()).To(BeFalse())
	})

	It("falls back to PATH when HERDLE_GH is unset", func() {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "gh")
		Expect(os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
		GinkgoT().Setenv("HERDLE_GH", "")                                   // empty -> Available consults PATH
		GinkgoT().Setenv("PATH", dir)
		Expect(vcs.NewGHRunner().Available()).To(BeTrue())
	})

	It("is false when gh is on neither HERDLE_GH nor PATH", func() {
		GinkgoT().Setenv("HERDLE_GH", "")
		GinkgoT().Setenv("PATH", GinkgoT().TempDir()) // empty dir, no gh
		Expect(vcs.NewGHRunner().Available()).To(BeFalse())
	})
})

var _ = Describe("GHRunner.Authenticated", func() {
	It("is true when `gh auth status` exits 0", func() {
		ghStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGHRunner().Authenticated()).To(BeTrue())
	})
	It("is false when `gh auth status` exits non-zero", func() {
		ghStub("#!/bin/sh\nexit 1\n")
		Expect(vcs.NewGHRunner().Authenticated()).To(BeFalse())
	})
	It("is false when gh is not installed", func() {
		os.Setenv("HERDLE_GH", filepath.Join(GinkgoT().TempDir(), "nope"))
		DeferCleanup(func() { os.Unsetenv("HERDLE_GH") })
		Expect(vcs.NewGHRunner().Authenticated()).To(BeFalse())
	})
})

var _ = Describe("GHRunner.KnownHosts", func() {
	writeHosts := func(body string) {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(body), 0o600)).To(Succeed())
		GinkgoT().Setenv("GH_CONFIG_DIR", dir)
	}

	It("returns just github.com when hosts.yml is absent", func() {
		GinkgoT().Setenv("GH_CONFIG_DIR", GinkgoT().TempDir()) // empty dir, no hosts.yml
		Expect(vcs.NewGHRunner().KnownHosts()).To(Equal([]string{"github.com"}))
	})

	It("unions github.com with the top-level host keys", func() {
		writeHosts("github.com:\n    user: x\ngithub.example.com:\n    user: y\n")
		Expect(vcs.NewGHRunner().KnownHosts()).To(ConsistOf("github.com", "github.example.com"))
	})

	It("ignores indented child keys and comment lines", func() {
		writeHosts("# a comment\ngithub.example.com:\n    oauth_token: abc\n    git_protocol: ssh\n")
		Expect(vcs.NewGHRunner().KnownHosts()).To(ConsistOf("github.com", "github.example.com"))
	})

	It("lowercases host keys for case-insensitive matching", func() {
		writeHosts("GitHub.Example.COM:\n    user: x\n")
		Expect(vcs.NewGHRunner().KnownHosts()).To(ConsistOf("github.com", "github.example.com"))
	})

	It("reads $XDG_CONFIG_HOME/gh/hosts.yml when GH_CONFIG_DIR is unset", func() {
		xdg := GinkgoT().TempDir()
		GinkgoT().Setenv("GH_CONFIG_DIR", "") // empty -> skip the GH_CONFIG_DIR branch
		GinkgoT().Setenv("XDG_CONFIG_HOME", xdg)
		ghDir := filepath.Join(xdg, "gh")
		Expect(os.MkdirAll(ghDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ghDir, "hosts.yml"),
			[]byte("github.enterprise.io:\n    user: z\n"), 0o600)).To(Succeed())
		Expect(vcs.NewGHRunner().KnownHosts()).To(ConsistOf("github.com", "github.enterprise.io"))
	})
})
