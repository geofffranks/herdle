package gate_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/gate"
)

var _ = Describe("ShouldEvaluate", func() {
	It("gates an Edit that writes pending-validation into a ticket", func() {
		p, g := gate.ShouldEvaluate(gate.HookInput{
			ToolName: "Edit", FilePath: "/repo/.tickets/her-5s12.md",
			WrittenText: "lifecycle: pending-validation\n"})
		Expect(g).To(BeTrue())
		Expect(p).To(Equal("/repo/.tickets/her-5s12.md"))
	})
	It("ignores a ticket edit not touching pending-validation", func() {
		_, g := gate.ShouldEvaluate(gate.HookInput{
			ToolName: "Write", FilePath: "/repo/.tickets/her-5s12.md",
			WrittenText: "lifecycle: in-development\n"})
		Expect(g).To(BeFalse())
	})
	It("ignores edits to non-ticket files", func() {
		_, g := gate.ShouldEvaluate(gate.HookInput{
			ToolName: "Edit", FilePath: "/repo/main.go",
			WrittenText: "lifecycle: pending-validation"})
		Expect(g).To(BeFalse())
	})
	It("gates a Bash sed that writes pending-validation into a ticket", func() {
		p, g := gate.ShouldEvaluate(gate.HookInput{
			ToolName: "Bash",
			Command:  `sed -i '' 's/^lifecycle:.*/lifecycle: pending-validation/' .tickets/her-5s12.md`})
		Expect(g).To(BeTrue())
		Expect(p).To(Equal(".tickets/her-5s12.md"))
	})
	It("does not gate a read-only grep mentioning pending-validation", func() {
		_, g := gate.ShouldEvaluate(gate.HookInput{
			ToolName: "Bash", Command: `grep pending-validation .tickets/her-5s12.md`})
		Expect(g).To(BeFalse())
	})
})

var _ = Describe("HasOverride", func() {
	It("honors the marker with a reason", func() {
		Expect(gate.HasOverride("lifecycle: pending-validation [skip-code-review-gate] hotfix")).To(BeTrue())
	})
	It("rejects a bare marker with no reason", func() {
		Expect(gate.HasOverride("[skip-code-review-gate]")).To(BeFalse())
	})
})

var _ = Describe("EffortsFromTranscript", func() {
	const ticket = "/repo/.tickets/her-5s12.md"

	skillLine := func(args string) string {
		return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"` + args + `"}}]}}`
	}
	userSlash := func(args string) string {
		return `{"type":"user","message":{"role":"user","content":"<command-name>/code-review</command-name>\n<command-args>` + args + `</command-args>"}}`
	}
	indevLine := func(path string) string {
		return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"` + path + `","new_string":"lifecycle: in-development\n"}}]}}`
	}

	It("counts medium and high from agent Skill tool_use", func() {
		tr := strings.NewReader(skillLine("feat/x medium --fix") + "\n" + skillLine("feat/x high --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["medium"]).To(BeTrue())
		Expect(e["high"]).To(BeTrue())
	})
	It("counts a user-typed /code-review slash command", func() {
		tr := strings.NewReader(userSlash("feat/x high --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["high"]).To(BeTrue())
		Expect(e["medium"]).To(BeFalse())
	})
	It("ignores reviews before this ticket's in-development marker", func() {
		tr := strings.NewReader(
			skillLine("feat/other medium --fix") + "\n" + // earlier feature
				indevLine(ticket) + "\n" +
				skillLine("feat/x high --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["medium"]).To(BeFalse()) // before the bound
		Expect(e["high"]).To(BeTrue())    // after the bound
	})
	It("falls back to session-wide when no in-development marker is present", func() {
		tr := strings.NewReader(skillLine("feat/x medium --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["medium"]).To(BeTrue())
	})
	It("survives malformed lines", func() {
		tr := strings.NewReader("not json\n" + skillLine("medium") + "\n{partial\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["medium"]).To(BeTrue())
	})
	It("does not count an effort word embedded in a branch name", func() {
		// only a medium pass ran, but the branch name contains "high"
		tr := strings.NewReader(skillLine("feat/high-priority medium --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ticket)
		Expect(e["medium"]).To(BeTrue())
		Expect(e["high"]).To(BeFalse()) // "high" in the branch name must not count
	})
	It("matches the in-development marker by ticket base name (relative vs absolute path)", func() {
		// the in-dev Edit recorded an absolute path; the gating target is the
		// relative path a Bash command would yield — the bound must still apply
		tr := strings.NewReader(
			skillLine("feat/other medium --fix") + "\n" + // earlier feature, before bound
				indevLine("/repo/.tickets/her-5s12.md") + "\n" +
				skillLine("feat/x high --fix") + "\n")
		e := gate.EffortsFromTranscript(tr, ".tickets/her-5s12.md")
		Expect(e["medium"]).To(BeFalse()) // excluded by the bound despite path-form mismatch
		Expect(e["high"]).To(BeTrue())
	})
	It("does not let a same-named ticket in another project bound this ticket", func() {
		// the in-dev marker is for a DIFFERENT project's ticket that happens to
		// share a file name; it must not bound this ticket's reviews
		tr := strings.NewReader(
			skillLine("feat/x medium --fix") + "\n" +
				indevLine("/projectB/.tickets/her-5s12.md") + "\n")
		e := gate.EffortsFromTranscript(tr, "/projectA/.tickets/her-5s12.md")
		Expect(e["medium"]).To(BeTrue()) // counted: projectB's marker does not bound projectA
	})
})

var _ = Describe("Decide", func() {
	const ticket = "/repo/.tickets/her-5s12.md"
	gatingInput := gate.HookInput{ToolName: "Edit", FilePath: ticket,
		WrittenText: "lifecycle: pending-validation\n"}
	both := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"medium"}}]}}` + "\n" +
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"high"}}]}}` + "\n"

	It("allows a non-gating edit regardless of transcript", func() {
		d := gate.Decide(gate.HookInput{ToolName: "Edit", FilePath: "/repo/main.go"}, nil)
		Expect(d.Allow).To(BeTrue())
	})
	It("allows when both passes are present", func() {
		d := gate.Decide(gatingInput, strings.NewReader(both))
		Expect(d.Allow).To(BeTrue())
	})
	It("blocks and names the missing pass", func() {
		one := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"medium"}}]}}` + "\n"
		d := gate.Decide(gatingInput, strings.NewReader(one))
		Expect(d.Allow).To(BeFalse())
		Expect(d.Missing).To(Equal([]string{"high"}))
		Expect(d.Reason).To(ContainSubstring("high"))
	})
	It("fails closed when the transcript is unreadable (nil)", func() {
		d := gate.Decide(gatingInput, nil)
		Expect(d.Allow).To(BeFalse())
	})
	It("allows a gating edit carrying the override marker with a reason", func() {
		in := gatingInput
		in.WrittenText += "[skip-code-review-gate] urgent hotfix\n"
		d := gate.Decide(in, nil)
		Expect(d.Allow).To(BeTrue())
	})
})
