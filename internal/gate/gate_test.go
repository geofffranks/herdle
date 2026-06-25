package gate_test

import (
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/gate"
)

var _ = Describe("ShouldEvaluate", func() {
	It("classifies an Edit that writes pending-validation", func() {
		p, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-5s12.md", WrittenText: "lifecycle: pending-validation\n"})
		Expect(t).To(Equal(gate.ToPendingValidation))
		Expect(p).To(Equal("/repo/.tickets/her-5s12.md"))
	})
	It("classifies an Edit that writes in-development", func() {
		p, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-5s12.md", WrittenText: "lifecycle: in-development\n"})
		Expect(t).To(Equal(gate.ToInDevelopment))
		Expect(p).To(Equal("/repo/.tickets/her-5s12.md"))
	})
	It("classifies an Edit that writes validated", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-5s12.md", WrittenText: "lifecycle: validated\n"})
		Expect(t).To(Equal(gate.ToValidated))
	})
	It("does not confuse pending-validation with validated", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her.md", WrittenText: "lifecycle: pending-validation\n"})
		Expect(t).To(Equal(gate.ToPendingValidation)) // not ToValidated
	})
	It("returns None for a ticket edit with no lifecycle marker", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her.md", WrittenText: "status: open\n"})
		Expect(t).To(Equal(gate.None))
	})
	It("returns None for edits to non-ticket files", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/main.go", WrittenText: "lifecycle: validated"})
		Expect(t).To(Equal(gate.None))
	})
	It("classifies a Bash sed that writes pending-validation", func() {
		p, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Bash",
			Command: `sed -i '' 's/^lifecycle:.*/lifecycle: pending-validation/' .tickets/her-5s12.md`})
		Expect(t).To(Equal(gate.ToPendingValidation))
		Expect(p).To(Equal(".tickets/her-5s12.md"))
	})
	It("returns None for a read-only grep mentioning a marker (no write indicator)", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Bash",
			Command: `grep "lifecycle: pending-validation" .tickets/her-5s12.md`})
		Expect(t).To(Equal(gate.None))
	})
	It("classifies a full-file Write by the frontmatter line, not a note mention", func() {
		// frontmatter sets pending-validation; a note in the body mentions a
		// different lifecycle value — the gate must follow the real line.
		body := "---\nid: her-x\nlifecycle: pending-validation\n---\n# T\n\n## Notes\nPlan written (lifecycle: validated noted here as prose).\n"
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Write",
			FilePath: "/repo/.tickets/her-x.md", WrittenText: body})
		Expect(t).To(Equal(gate.ToPendingValidation)) // not ToValidated from the note
	})
	It("returns None for an edit that only mentions a lifecycle value in prose", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-x.md", WrittenText: "see (lifecycle: validated) above\n"})
		Expect(t).To(Equal(gate.None))
	})
	It("returns None for a bogus suffixed lifecycle value (not a real state)", func() {
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-x.md", WrittenText: "lifecycle: validated-ish\n"})
		Expect(t).To(Equal(gate.None))
	})
	It("returns None when a frontmatter lifecycle line has trailing prose", func() {
		// the value must be the whole rest of the line, not a prefix
		_, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Edit",
			FilePath: "/repo/.tickets/her-x.md", WrittenText: "lifecycle: validated is the goal\n"})
		Expect(t).To(Equal(gate.None))
	})
	It("classifies a Bash write of validated into a ticket", func() {
		p, t := gate.ShouldEvaluate(gate.HookInput{ToolName: "Bash",
			Command: `printf 'lifecycle: validated\n' > .tickets/her-x.md`})
		Expect(t).To(Equal(gate.ToValidated))
		Expect(p).To(Equal(".tickets/her-x.md"))
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

var _ = Describe("OpenItemCount", func() {
	It("counts unchecked boxes and ignores checked ones", func() {
		doc := "- [x] done\n- [ ] todo\n* [ ] another\n+ [X] also done\n"
		Expect(gate.OpenItemCount(doc)).To(Equal(2))
	})
	It("counts indented unchecked boxes", func() {
		Expect(gate.OpenItemCount("  - [ ] nested\n")).To(Equal(1))
	})
	It("ignores checkboxes inside fenced code blocks", func() {
		doc := "- [ ] real\n```\n- [ ] example in code\n```\n- [x] closed\n"
		Expect(gate.OpenItemCount(doc)).To(Equal(1))
	})
	It("returns zero when every box is checked", func() {
		Expect(gate.OpenItemCount("- [x] all\n- [X] done\n")).To(Equal(0))
	})
	It("returns zero for a doc with no task items", func() {
		Expect(gate.OpenItemCount("# Title\n\nprose only\n")).To(Equal(0))
	})
})

var _ = Describe("Decide", func() {
	const ticket = "/repo/.tickets/her-5s12.md"
	skill := func(args string) string {
		return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"` + args + `"}}]}}`
	}
	bothReviews := skill("medium") + "\n" + skill("high") + "\n"

	Describe("pending-validation", func() {
		in := gate.HookInput{ToolName: "Edit", FilePath: ticket, WrittenText: "lifecycle: pending-validation\n"}
		env := func(tr io.Reader) gate.Env {
			return gate.Env{Transition: gate.ToPendingValidation, TicketPath: ticket, Transcript: tr}
		}
		It("allows a non-gating call", func() {
			Expect(gate.Decide(gate.HookInput{}, gate.Env{Transition: gate.None}).Allow).To(BeTrue())
		})
		It("allows when both passes are present", func() {
			Expect(gate.Decide(in, env(strings.NewReader(bothReviews))).Allow).To(BeTrue())
		})
		It("blocks and names the missing pass", func() {
			d := gate.Decide(in, env(strings.NewReader(skill("medium")+"\n")))
			Expect(d.Allow).To(BeFalse())
			Expect(d.Missing).To(Equal([]string{"high"}))
		})
		It("fails closed on a nil transcript", func() {
			Expect(gate.Decide(in, env(nil)).Allow).To(BeFalse())
		})
		It("honors the override", func() {
			ov := in
			ov.WrittenText += "[skip-code-review-gate] hotfix\n"
			Expect(gate.Decide(ov, env(nil)).Allow).To(BeTrue())
		})
		envDisk := func(lifecycle string) gate.Env {
			return gate.Env{Transition: gate.ToPendingValidation, TicketPath: ticket,
				TicketContent: "lifecycle: " + lifecycle + "\n", TicketReadOK: true}
		}
		It("allows a backward rollback from validated without a transcript", func() {
			Expect(gate.Decide(in, envDisk("validated")).Allow).To(BeTrue())
		})
		It("allows an idempotent re-write at pending-validation", func() {
			Expect(gate.Decide(in, envDisk("pending-validation")).Allow).To(BeTrue())
		})
		It("still gates a forward bump from in-development (no rollback)", func() {
			e := envDisk("in-development")
			e.Transcript = strings.NewReader(skill("medium") + "\n") // only one pass
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("still fails closed for a readable non-rollback ticket with no transcript", func() {
			// readable on-disk in-development (not a rollback state) + nil transcript:
			// the short-circuit must miss and the fail-closed forward gate must fire.
			// Distinct from the unreadable-ticket "fails closed on a nil transcript" case.
			Expect(gate.Decide(in, envDisk("in-development")).Allow).To(BeFalse())
		})
	})

	Describe("validated", func() {
		in := gate.HookInput{ToolName: "Edit", FilePath: ticket, WrittenText: "lifecycle: validated\n"}
		base := func() gate.Env {
			return gate.Env{Transition: gate.ToValidated, TicketPath: ticket,
				TicketContent: "lifecycle: pending-validation\n", TicketReadOK: true,
				ValidationFound: true, ValidationDocs: []string{"- [x] done\n"}}
		}
		It("allows when pending-validation and every box checked", func() {
			Expect(gate.Decide(in, base()).Allow).To(BeTrue())
		})
		It("blocks when a validation box is open", func() {
			e := base()
			e.ValidationDocs = []string{"- [x] auto\n- [ ] human\n"}
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("blocks (monotonic) when the ticket is still in-development", func() {
			e := base()
			e.TicketContent = "lifecycle: in-development\n"
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("allows an idempotent re-validation", func() {
			e := base()
			e.TicketContent = "lifecycle: validated\n"
			e.ValidationFound = false // unread: idempotent path must not require a doc
			Expect(gate.Decide(in, e).Allow).To(BeTrue())
		})
		It("fails closed when no validation doc exists", func() {
			e := base()
			e.ValidationFound = false
			e.ValidationDocs = nil
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("fails closed when the ticket is unreadable", func() {
			e := base()
			e.TicketReadOK = false
			e.TicketContent = ""
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("honors the override even when otherwise blocked", func() {
			ov := in
			ov.WrittenText += "[skip-validation-gate] signed off offline\n"
			e := base()
			e.TicketContent = "lifecycle: in-development\n"
			Expect(gate.Decide(ov, e).Allow).To(BeTrue())
		})
	})

	Describe("in-development", func() {
		in := gate.HookInput{ToolName: "Edit", FilePath: ticket, WrittenText: "lifecycle: in-development\n"}
		It("blocks when no branch/external-ref is present anywhere", func() {
			e := gate.Env{Transition: gate.ToInDevelopment, TicketPath: ticket, TicketReadOK: true, TicketContent: "id: x\n"}
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
		It("allows when branch: is added in the same edit", func() {
			ov := in
			ov.WrittenText += "branch: feat/x\n"
			e := gate.Env{Transition: gate.ToInDevelopment, TicketPath: ticket, TicketReadOK: true, TicketContent: "id: x\n"}
			Expect(gate.Decide(ov, e).Allow).To(BeTrue())
		})
		It("allows when external-ref is already on the on-disk ticket", func() {
			e := gate.Env{Transition: gate.ToInDevelopment, TicketPath: ticket, TicketReadOK: true, TicketContent: "external-ref: gh-12\n"}
			Expect(gate.Decide(in, e).Allow).To(BeTrue())
		})
		It("allows when the [skip-branch-linkage] override is present with a reason", func() {
			ov := in
			ov.WrittenText += "[skip-branch-linkage] tracked elsewhere\n"
			e := gate.Env{Transition: gate.ToInDevelopment, TicketPath: ticket, TicketReadOK: true, TicketContent: "id: x\n"}
			Expect(gate.Decide(ov, e).Allow).To(BeTrue())
		})
		It("blocks when the ticket is unreadable and no link is in the edit", func() {
			e := gate.Env{Transition: gate.ToInDevelopment, TicketPath: ticket, TicketReadOK: false}
			Expect(gate.Decide(in, e).Allow).To(BeFalse())
		})
	})
})
