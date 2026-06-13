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
})
