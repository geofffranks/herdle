package vcs_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
)

// glabStub writes an executable `glab` stub, points HERDLE_GLAB at it, returns its
// dir (handy for retry-counter / args state).
func glabStub(body string) string {
	dir := GinkgoT().TempDir()
	p := filepath.Join(dir, "glab")
	Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_GLAB", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_GLAB") })
	return dir
}

var _ = Describe("GLRunner.PRList", func() {
	var gl vcs.GLRunner
	BeforeEach(func() { gl = vcs.NewGLRunner() })

	It("maps a GitLab MR (iid/source_branch/opened state) onto the neutral PR", func() {
		glabStub(`#!/bin/sh
echo '[{"iid":12,"state":"opened","source_branch":"feat/x","title":"Add x","detailed_merge_status":"mergeable"}]'
`)
		prs, err := gl.PRList("grp/proj", "open")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(Equal([]vcs.PR{{
			Number: 12, State: "OPEN", HeadRefName: "feat/x", Title: "Add x", Mergeable: "MERGEABLE",
		}}))
	})

	It("maps merge/closed/locked states and draft/conflict/changes-requested fields", func() {
		glabStub(`#!/bin/sh
echo '[
  {"iid":1,"state":"merged","source_branch":"a"},
  {"iid":2,"state":"closed","source_branch":"b"},
  {"iid":3,"state":"locked","source_branch":"c"},
  {"iid":4,"state":"opened","source_branch":"d","draft":true},
  {"iid":5,"state":"opened","source_branch":"e","work_in_progress":true},
  {"iid":6,"state":"opened","source_branch":"f","has_conflicts":true},
  {"iid":7,"state":"opened","source_branch":"g","detailed_merge_status":"conflict"},
  {"iid":8,"state":"opened","source_branch":"h","detailed_merge_status":"requested_changes"},
  {"iid":9,"state":"opened","source_branch":"i","detailed_merge_status":"broken_status"},
  {"iid":10,"state":"opened","source_branch":"j","detailed_merge_status":"not_approved"},
  {"iid":11,"state":"opened","source_branch":"k","detailed_merge_status":"need_rebase"},
  {"iid":12,"state":"opened","source_branch":"l","detailed_merge_status":"discussions_not_resolved"},
  {"iid":13,"state":"opened","source_branch":"m","detailed_merge_status":"ci_must_pass"},
  {"iid":14,"state":"opened","source_branch":"n","detailed_merge_status":"ci_still_running"}
]'
`)
		prs, err := gl.PRList("grp/proj", "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(HaveLen(14))
		Expect(prs[0].State).To(Equal("MERGED"))
		Expect(prs[1].State).To(Equal("CLOSED"))
		Expect(prs[2].State).To(Equal("CLOSED"))
		Expect(prs[3].IsDraft).To(BeTrue())
		Expect(prs[4].IsDraft).To(BeTrue())
		Expect(prs[5].Mergeable).To(Equal("CONFLICTING"))
		Expect(prs[6].Mergeable).To(Equal("CONFLICTING"))
		Expect(prs[7].ReviewDecision).To(Equal("CHANGES_REQUESTED"))
		// broken_status means GitLab couldn't compute mergeability (transient), not
		// a real conflict -> must NOT be marked CONFLICTING (neutral instead).
		Expect(prs[8].Mergeable).To(BeEmpty())
		Expect(prs[8].BlockReason).To(BeEmpty())
		// Named non-hard blockers -> BlockReason (no conflict/changes-requested).
		Expect(prs[9].BlockReason).To(Equal("needs approval"))
		Expect(prs[10].BlockReason).To(Equal("needs rebase"))
		Expect(prs[11].BlockReason).To(Equal("unresolved threads"))
		Expect(prs[12].BlockReason).To(Equal("checks not passed"))
		// ci_still_running is a transient "computing" state -> stays neutral.
		Expect(prs[13].BlockReason).To(BeEmpty())
		Expect(prs[13].Mergeable).To(BeEmpty())
	})

	It("treats an empty JSON array as zero MRs (no error)", func() {
		glabStub("#!/bin/sh\necho '[]'\n")
		prs, err := gl.PRList("grp/proj", "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(BeEmpty())
	})

	It("adds --all only for the all state", func() {
		dir := glabStub(`#!/bin/sh
echo "$@" > "$0.args"
echo '[]'
`)
		_, err := gl.PRList("grp/proj", "all")
		Expect(err).NotTo(HaveOccurred())
		args, _ := os.ReadFile(filepath.Join(dir, "glab.args"))
		Expect(string(args)).To(ContainSubstring("--all"))
		Expect(string(args)).NotTo(ContainSubstring("--state opened"))
		Expect(string(args)).To(ContainSubstring("--author @me"))
		Expect(string(args)).To(ContainSubstring("-R grp/proj"))

		_, err = gl.PRList("grp/proj", "open")
		Expect(err).NotTo(HaveOccurred())
		args, _ = os.ReadFile(filepath.Join(dir, "glab.args"))
		Expect(string(args)).NotTo(ContainSubstring("--all"))
		Expect(string(args)).To(ContainSubstring("--state opened")) // open path pins state explicitly
	})

	It("retries once then succeeds", func() {
		dir := glabStub(`#!/bin/sh
c="$0.n"; n=$(cat "$c" 2>/dev/null || echo 0); echo $((n+1)) > "$c"
if [ "$n" = "0" ]; then echo "transient" >&2; exit 1; fi
echo '[]'
`)
		_, err := gl.PRList("grp/proj", "open")
		Expect(err).NotTo(HaveOccurred())
		data, _ := os.ReadFile(filepath.Join(dir, "glab.n"))
		Expect(strings.TrimSpace(string(data))).To(Equal("2"))
	})

	It("returns an error (not empty) when glab fails both attempts", func() {
		glabStub("#!/bin/sh\necho boom >&2\nexit 1\n")
		_, err := gl.PRList("grp/proj", "open")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when output is not a JSON array", func() {
		glabStub("#!/bin/sh\necho 'not json'\n")
		_, err := gl.PRList("grp/proj", "open")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the glab binary is unavailable", func() {
		os.Setenv("HERDLE_GLAB", "/nonexistent/glab-binary")
		DeferCleanup(func() { os.Unsetenv("HERDLE_GLAB") })
		_, err := gl.PRList("grp/proj", "open")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("GLRunner.IssueList", func() {
	var gl vcs.GLRunner
	BeforeEach(func() { gl = vcs.NewGLRunner() })

	It("maps GitLab iid/state onto the neutral Issue", func() {
		glabStub(`#!/bin/sh
echo '[{"iid":42,"title":"Broken export","state":"opened"}]'
`)
		issues, err := gl.IssueList("grp/proj", "open")
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(Equal([]vcs.Issue{{Number: 42, Title: "Broken export", State: "OPEN"}}))
	})

	It("maps a closed GitLab state to CLOSED", func() {
		glabStub(`#!/bin/sh
echo '[{"iid":7,"title":"Old bug","state":"closed"}]'
`)
		issues, err := gl.IssueList("grp/proj", "all")
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(Equal([]vcs.Issue{{Number: 7, Title: "Old bug", State: "CLOSED"}}))
	})

	It("treats an empty array as zero issues (no error)", func() {
		glabStub("#!/bin/sh\necho '[]'\n")
		issues, err := gl.IssueList("grp/proj", "open")
		Expect(err).NotTo(HaveOccurred())
		Expect(issues).To(BeEmpty())
	})
})

var _ = Describe("GLRunner.Available / Authenticated", func() {
	It("Available is true when HERDLE_GLAB points at an existing binary", func() {
		glabStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGLRunner().Available()).To(BeTrue())
	})
	It("Available is false when HERDLE_GLAB points at a missing path", func() {
		os.Setenv("HERDLE_GLAB", filepath.Join(GinkgoT().TempDir(), "nope-glab"))
		DeferCleanup(func() { os.Unsetenv("HERDLE_GLAB") })
		Expect(vcs.NewGLRunner().Available()).To(BeFalse())
	})
	It("Authenticated is true when `glab auth status` exits 0", func() {
		glabStub("#!/bin/sh\nexit 0\n")
		Expect(vcs.NewGLRunner().Authenticated()).To(BeTrue())
	})
	It("Authenticated is false when `glab auth status` exits non-zero", func() {
		glabStub("#!/bin/sh\nexit 1\n")
		Expect(vcs.NewGLRunner().Authenticated()).To(BeFalse())
	})
})

var _ = Describe("GLRunner.KnownHosts", func() {
	writeConfig := func(body string) {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o600)).To(Succeed())
		GinkgoT().Setenv("GLAB_CONFIG_DIR", dir)
	}

	It("returns just gitlab.com when config.yml is absent", func() {
		GinkgoT().Setenv("GLAB_CONFIG_DIR", GinkgoT().TempDir()) // empty dir, no config.yml
		Expect(vcs.NewGLRunner().KnownHosts()).To(Equal([]string{"gitlab.com"}))
	})

	It("unions gitlab.com with the host keys under the hosts: map", func() {
		writeConfig("host: gitlab.com\nhosts:\n    gitlab.com:\n        user: a\n    gitlab.enterprise.io:\n        user: b\n")
		Expect(vcs.NewGLRunner().KnownHosts()).To(ConsistOf("gitlab.com", "gitlab.enterprise.io"))
	})

	It("ignores a host's own indented settings and trailing top-level keys", func() {
		writeConfig("hosts:\n    gitlab.example.com:\n        api_host: gitlab.example.com\n        token: secret\nlast_seen_version: v1\n")
		Expect(vcs.NewGLRunner().KnownHosts()).To(ConsistOf("gitlab.com", "gitlab.example.com"))
	})

	It("reads tab-indented host entries (YAML forbids tabs, but real configs have them)", func() {
		writeConfig("hosts:\n\tgitlab.corp.io:\n\t\tuser: a\n")
		Expect(vcs.NewGLRunner().KnownHosts()).To(ConsistOf("gitlab.com", "gitlab.corp.io"))
	})

	It("lowercases host keys for case-insensitive matching", func() {
		writeConfig("hosts:\n    GitLab.Corp.IO:\n        user: a\n")
		Expect(vcs.NewGLRunner().KnownHosts()).To(ConsistOf("gitlab.com", "gitlab.corp.io"))
	})
})
