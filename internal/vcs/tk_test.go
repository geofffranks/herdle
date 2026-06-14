package vcs_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
)

// tkStub writes an executable `tk` stub, points HERDLE_TK at it, returns its dir.
func tkStub(body string) string {
	dir := GinkgoT().TempDir()
	p := filepath.Join(dir, "tk")
	Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_TK", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_TK") })
	return dir
}

var _ = Describe("TKRunner.Available", func() {
	It("is true when HERDLE_TK points at an existing file", func() {
		tkStub("#!/bin/sh\n:\n")
		Expect(vcs.NewTKRunner().Available()).To(BeTrue())
	})
	It("is false when HERDLE_TK points at a missing path", func() {
		os.Setenv("HERDLE_TK", filepath.Join(GinkgoT().TempDir(), "nope"))
		DeferCleanup(func() { os.Unsetenv("HERDLE_TK") })
		Expect(vcs.NewTKRunner().Available()).To(BeFalse())
	})
})

// tkRepo builds a temp repo dir with a .tickets/ dir, writes a `tk` stub that
// emits the given query output, points HERDLE_TK at it, and returns the dir.
func tkRepo(queryOut string) string {
	dir := GinkgoT().TempDir()
	Expect(os.MkdirAll(filepath.Join(dir, ".tickets"), 0o755)).To(Succeed())
	stub := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  query) cat <<'EOF'\n" + queryOut + "EOF\n  ;;\n" +
		"  ready) printf 'her-2 [open] - second\\nher-3 [in_progress] - third\\n' ;;\n" +
		"esac\n"
	p := filepath.Join(dir, "tk")
	Expect(os.WriteFile(p, []byte(stub), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_TK", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_TK") })
	return dir
}

// customTK writes a tk stub with a verbatim script body (used for error paths)
// in a fresh temp repo with a .tickets/ dir, points HERDLE_TK at it.
func customTK(body string) string {
	dir := GinkgoT().TempDir()
	Expect(os.MkdirAll(filepath.Join(dir, ".tickets"), 0o755)).To(Succeed())
	p := filepath.Join(dir, "tk")
	Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- executable stub
	os.Setenv("HERDLE_TK", p)
	DeferCleanup(func() { os.Unsetenv("HERDLE_TK") })
	return dir
}

var _ = Describe("TKRunner", func() {
	var tk vcs.TKRunner
	BeforeEach(func() { tk = vcs.NewTKRunner() })

	It("parses tk query NDJSON, converts priority, and reads the title heading", func() {
		dir := tkRepo(`{"id":"her-2","status":"open","lifecycle":"-","priority":"3","branch":"b","external-ref":"gh-9","type":"task","assignee":"Geoff"}
`)
		Expect(os.WriteFile(filepath.Join(dir, ".tickets", "her-2.md"),
			[]byte("---\nid: her-2\n---\n# Second ticket title\n\nbody\n"), 0o644)).To(Succeed())

		tickets, err := tk.Tickets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tickets).To(Equal([]vcs.Ticket{{
			ID: "her-2", Status: "open", Lifecycle: "-", Title: "Second ticket title",
			Branch: "b", ExternalRef: "gh-9", Type: "task", Assignee: "Geoff", Priority: 3,
		}}))
	})

	It("returns an empty title when the ticket file is missing", func() {
		dir := tkRepo(`{"id":"her-x","status":"open","priority":"2"}
`)
		tickets, err := tk.Tickets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tickets).To(HaveLen(1))
		Expect(tickets[0].Title).To(Equal(""))
	})

	It("extracts ready ticket ids from tk ready", func() {
		dir := tkRepo(`{"id":"her-2","status":"open","priority":"2"}
`)
		ids, err := tk.Ready(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(ids).To(Equal([]string{"her-2", "her-3"}))
	})

	It("reports HasTickets true when .tickets/ exists, false otherwise", func() {
		dir := tkRepo(`{"id":"her-2","status":"open","priority":"2"}
`)
		Expect(tk.HasTickets(dir)).To(BeTrue())

		bare := GinkgoT().TempDir()
		Expect(tk.HasTickets(bare)).To(BeFalse())
	})

	It("skips malformed query rows that have no id", func() {
		dir := tkRepo(`{"status":"open","priority":"2"}
`)
		tickets, err := tk.Tickets(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(tickets).To(BeEmpty())
	})

	It("returns an error when tk query exits non-zero", func() {
		dir := customTK("#!/bin/sh\necho 'db locked' >&2\nexit 1\n")
		_, err := tk.Tickets(dir)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when tk ready exits non-zero", func() {
		dir := customTK("#!/bin/sh\necho boom >&2\nexit 1\n")
		_, err := tk.Ready(dir)
		Expect(err).To(HaveOccurred())
	})
})
