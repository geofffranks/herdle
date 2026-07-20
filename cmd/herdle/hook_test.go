package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/gate"
)

func init() {
	// --agent is an internal hook-protocol selector, not user-facing CLI surface.
	// The docs drift guard's registry predates hidden flags and does not inspect
	// cli.Flag.IsVisible, so classify this hidden protocol flag with its builtins.
	docsBuiltinFlags["--agent"] = true
}

var _ = Describe("hookCommand", func() {
	gatekeeper := func() *cli.Command {
		for _, c := range hookCommand().Subcommands {
			if c.Name == "gatekeeper" {
				return c
			}
		}
		return nil
	}

	It("keeps code-review-gate as an alias of gatekeeper (upgrade migration window)", func() {
		gk := gatekeeper()
		Expect(gk).NotTo(BeNil())
		Expect(gk.Aliases).To(ContainElement("code-review-gate"))
	})

	It("keeps the hook agent selector hidden and defaults it to Claude", func() {
		gk := gatekeeper()
		Expect(gk).NotTo(BeNil())
		Expect(gk.Flags).To(HaveLen(1))
		flag, ok := gk.Flags[0].(*cli.StringFlag)
		Expect(ok).To(BeTrue())
		Expect(flag.Name).To(Equal("agent"))
		Expect(flag.Hidden).To(BeTrue())
		Expect(flag.Value).To(Equal(string(agent.Claude)))
	})
})

var _ = Describe("runGatekeeper Claude compatibility", func() {
	runGatekeeper := func(r io.Reader) gate.Decision {
		return runGatekeeper(r, agent.Claude, "")
	}
	writeTranscript := func(lines ...string) string {
		dir := GinkgoT().TempDir()
		p := filepath.Join(dir, "transcript.jsonl")
		Expect(os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600)).To(Succeed())
		return p
	}
	skill := func(args string) string {
		return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"code-review","args":"` + args + `"}}]}}`
	}
	stdin := func(toolInput, transcript string) io.Reader {
		return strings.NewReader(`{"tool_name":"Edit","tool_input":` + toolInput + `,"transcript_path":"` + transcript + `"}`)
	}

	Describe("pending-validation", func() {
		It("allows when both passes are present", func() {
			tp := writeTranscript(skill("medium"), skill("high"))
			ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
			Expect(runGatekeeper(stdin(ti, tp)).Allow).To(BeTrue())
		})
		It("blocks when a pass is missing", func() {
			tp := writeTranscript(skill("medium"))
			ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
			Expect(runGatekeeper(stdin(ti, tp)).Allow).To(BeFalse())
		})
		It("fails closed when the transcript path is missing", func() {
			ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
			Expect(runGatekeeper(stdin(ti, "/no/such/transcript.jsonl")).Allow).To(BeFalse())
		})
		It("allows on malformed stdin (fail-open on envelope parse)", func() {
			Expect(runGatekeeper(strings.NewReader("not json")).Allow).To(BeTrue())
		})

		writeRepoTicket := func(lifecycle string) (repo, ticket string) {
			repo = GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(repo, ".tickets"), 0o750)).To(Succeed())
			ticket = filepath.Join(repo, ".tickets", "her-tolc.md")
			Expect(os.WriteFile(ticket, []byte("---\nid: her-tolc\nlifecycle: "+lifecycle+"\n---\n"), 0o600)).To(Succeed())
			return repo, ticket
		}
		// stdinNoTranscript builds a pending-validation Edit with a cwd (so the
		// on-disk ticket resolves) but no transcript — a rollback must allow anyway.
		stdinNoTranscript := func(repo, ticket string) io.Reader {
			ns, _ := json.Marshal("lifecycle: pending-validation\n")
			ti := `{"file_path":"` + ticket + `","new_string":` + string(ns) + `}`
			return strings.NewReader(`{"tool_name":"Edit","tool_input":` + ti + `,"cwd":"` + repo + `"}`)
		}
		It("allows a rollback from validated with no transcript", func() {
			repo, ticket := writeRepoTicket("validated")
			Expect(runGatekeeper(stdinNoTranscript(repo, ticket)).Allow).To(BeTrue())
		})
		It("still blocks a forward bump from in-development missing a pass", func() {
			repo, ticket := writeRepoTicket("in-development")
			tp := writeTranscript(skill("medium")) // only one pass
			ns, _ := json.Marshal("lifecycle: pending-validation\n")
			ti := `{"file_path":"` + ticket + `","new_string":` + string(ns) + `}`
			in := strings.NewReader(`{"tool_name":"Edit","tool_input":` + ti + `,"cwd":"` + repo + `","transcript_path":"` + tp + `"}`)
			Expect(runGatekeeper(in).Allow).To(BeFalse())
		})
	})

	Describe("validated", func() {
		// writeRepo returns the per-spec repo+ticket paths (no shared mutable
		// state across It blocks).
		writeRepo := func(lifecycle, validation string) (repo, ticket string) {
			repo = GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(repo, ".tickets"), 0o750)).To(Succeed())
			ticket = filepath.Join(repo, ".tickets", "her-a4lq.md")
			Expect(os.WriteFile(ticket, []byte("---\nid: her-a4lq\nlifecycle: "+lifecycle+"\n---\n"), 0o600)).To(Succeed())
			if validation != "" {
				vdir := filepath.Join(repo, "docs", "superpowers", "validation")
				Expect(os.MkdirAll(vdir, 0o750)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(vdir, "2026-06-21-her-a4lq-x-validation.md"), []byte(validation), 0o600)).To(Succeed())
			}
			return repo, ticket
		}
		run := func(repo, ticket, extra string) gate.Decision {
			ns, _ := json.Marshal("lifecycle: validated\n" + extra)
			ti := `{"file_path":"` + ticket + `","new_string":` + string(ns) + `}`
			return runGatekeeper(strings.NewReader(`{"tool_name":"Edit","tool_input":` + ti + `,"cwd":"` + repo + `"}`))
		}
		It("blocks when the validation doc has open items", func() {
			repo, ticket := writeRepo("pending-validation", "- [x] auto\n- [ ] human\n")
			Expect(run(repo, ticket, "").Allow).To(BeFalse())
		})
		It("allows when every box is checked", func() {
			repo, ticket := writeRepo("pending-validation", "- [x] auto\n- [x] human\n")
			Expect(run(repo, ticket, "").Allow).To(BeTrue())
		})
		It("blocks (monotonic) when the ticket is not yet pending-validation", func() {
			repo, ticket := writeRepo("in-development", "- [x] all\n")
			Expect(run(repo, ticket, "").Allow).To(BeFalse())
		})
		It("fails closed when no validation doc exists", func() {
			repo, ticket := writeRepo("pending-validation", "")
			Expect(run(repo, ticket, "").Allow).To(BeFalse())
		})
		It("allows with the override marker", func() {
			repo, ticket := writeRepo("in-development", "- [ ] human\n")
			Expect(run(repo, ticket, "[skip-validation-gate] signed off offline\n").Allow).To(BeTrue())
		})
		It("reads a feature-dir validation doc (docs/superpowers/<tkid>-<slug>/validation.md)", func() {
			repo := GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(repo, ".tickets"), 0o750)).To(Succeed())
			ticket := filepath.Join(repo, ".tickets", "her-a4lq.md")
			Expect(os.WriteFile(ticket, []byte("---\nid: her-a4lq\nlifecycle: pending-validation\n---\n"), 0o600)).To(Succeed())
			fdir := filepath.Join(repo, "docs", "superpowers", "her-a4lq-merged-flow")
			Expect(os.MkdirAll(fdir, 0o750)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(fdir, "validation.md"), []byte("- [x] auto\n- [x] human\n"), 0o600)).To(Succeed())
			Expect(run(repo, ticket, "").Allow).To(BeTrue())
		})
	})

	Describe("in-development", func() {
		writeTicket := func(body string) (repo, ticket string) {
			repo = GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(repo, ".tickets"), 0o750)).To(Succeed())
			ticket = filepath.Join(repo, ".tickets", "her-x.md")
			Expect(os.WriteFile(ticket, []byte(body), 0o600)).To(Succeed())
			return repo, ticket
		}
		run := func(repo, ticket, newString string) gate.Decision {
			ns, _ := json.Marshal(newString)
			ti := `{"file_path":"` + ticket + `","new_string":` + string(ns) + `}`
			return runGatekeeper(strings.NewReader(`{"tool_name":"Edit","tool_input":` + ti + `,"cwd":"` + repo + `"}`))
		}
		It("blocks when the ticket has no branch/external-ref", func() {
			repo, ticket := writeTicket("---\nid: her-x\nstatus: open\n---\n")
			Expect(run(repo, ticket, "lifecycle: in-development\n").Allow).To(BeFalse())
		})
		It("allows when branch: is added in the same edit", func() {
			repo, ticket := writeTicket("---\nid: her-x\n---\n")
			Expect(run(repo, ticket, "lifecycle: in-development\nbranch: feat/her-x\n").Allow).To(BeTrue())
		})
		It("allows when branch: is already on the on-disk ticket", func() {
			repo, ticket := writeTicket("---\nid: her-x\nbranch: feat/her-x\n---\n")
			Expect(run(repo, ticket, "lifecycle: in-development\n").Allow).To(BeTrue())
		})
	})
})

var _ = Describe("runGatekeeper Polytoken", func() {
	const completeReview = "## Herdle code review\n\n" +
		"- [x] Standard review completed\n" +
		"- [x] Standard review findings addressed\n" +
		"- [x] Deep review completed\n" +
		"- [x] Deep review findings addressed\n"

	writeProject := func(lifecycle, review string) string {
		root := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(root, ".tickets"), 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(root, ".tickets", "her-x.md"),
			[]byte("---\nid: her-x\nlifecycle: "+lifecycle+"\nbranch: feat/x\n---\n"), 0o600)).To(Succeed())
		if review != "" {
			dir := filepath.Join(root, "docs", "superpowers", "validation")
			Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "2026-06-22-her-x-validation.md"), []byte(review), 0o600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "2026-06-22-her-other-validation.md"), []byte(completeReview), 0o600)).To(Succeed())
		}
		return root
	}

	DescribeTable("normalizes native tools and resolves relative ticket paths from projectDir",
		func(payload string) {
			root := writeProject("in-development", completeReview)
			d := runGatekeeper(strings.NewReader(payload), agent.Polytoken, root)
			Expect(d.Allow).To(BeTrue(), d.Reason)
		},
		Entry("file_edit_search_replace", `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`),
		Entry("file_write", `{"tool_name":"file_write","input":{"path":".tickets/her-x.md","content":"lifecycle: pending-validation\n"}}`),
		Entry("shell_exec", `{"tool_name":"shell_exec","input":{"command":"sed -i '' 's/^lifecycle:.*/lifecycle: pending-validation/' .tickets/her-x.md"}}`),
	)

	It("blocks pending-validation when a correlated marker is missing", func() {
		root := writeProject("in-development", strings.Replace(completeReview, "- [x] Deep review findings addressed\n", "", 1))
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		d := runGatekeeper(strings.NewReader(payload), agent.Polytoken, root)
		Expect(d.Allow).To(BeFalse())
		Expect(d.Missing).To(Equal([]string{"deep-addressed"}))
	})

	It("accepts required markers split across multiple correlated docs", func() {
		root := writeProject("in-development", "")
		dir := filepath.Join(root, "docs", "superpowers", "validation")
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "a-her-x.md"), []byte("- [x] Standard review completed\n- [x] Standard review findings addressed\n"), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "b-her-x.md"), []byte("- [x] Deep review completed\n- [x] Deep review findings addressed\n"), 0o600)).To(Succeed())
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		Expect(runGatekeeper(strings.NewReader(payload), agent.Polytoken, root).Allow).To(BeTrue())
	})

	It("rejects a marker duplicated across correlated docs", func() {
		root := writeProject("in-development", completeReview)
		dir := filepath.Join(root, "docs", "superpowers", "validation")
		Expect(os.WriteFile(filepath.Join(dir, "duplicate-her-x.md"), []byte("- [x] Standard review completed\n"), 0o600)).To(Succeed())
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		d := runGatekeeper(strings.NewReader(payload), agent.Polytoken, root)
		Expect(d.Allow).To(BeFalse())
		Expect(d.Missing).To(ContainElement("standard-completed"))
	})

	It("fails closed when any correlated doc is unreadable even if another is complete", func() {
		root := writeProject("in-development", completeReview)
		dir := filepath.Join(root, "docs", "superpowers", "validation", "unreadable-her-x.md")
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		d := runGatekeeper(strings.NewReader(payload), agent.Polytoken, root)
		Expect(d.Allow).To(BeFalse())
		Expect(d.Reason).To(ContainSubstring("cannot read the ticket-correlated validation doc"))
	})

	Describe("validated transition validation-doc readability", func() {
		const payload = `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: validated\n"}}`

		It("allows a readable complete correlated doc", func() {
			root := writeProject("pending-validation", "- [x] acceptance complete\n")
			Expect(runGatekeeper(strings.NewReader(payload), agent.Polytoken, root).Allow).To(BeTrue())
		})

		It("denies an unreadable-only correlated match", func() {
			root := writeProject("pending-validation", "")
			unreadable := filepath.Join(root, "docs", "superpowers", "validation", "unreadable-her-x.md")
			Expect(os.MkdirAll(unreadable, 0o750)).To(Succeed())
			Expect(runGatekeeper(strings.NewReader(payload), agent.Polytoken, root).Allow).To(BeFalse())
		})

		It("denies mixed readable and unreadable correlated matches", func() {
			root := writeProject("pending-validation", "- [x] acceptance complete\n")
			unreadable := filepath.Join(root, "docs", "superpowers", "validation", "unreadable-her-x.md")
			Expect(os.MkdirAll(unreadable, 0o750)).To(Succeed())
			Expect(runGatekeeper(strings.NewReader(payload), agent.Polytoken, root).Allow).To(BeFalse())
		})
	})

	It("fails open for malformed payloads and irrelevant tools", func() {
		Expect(runGatekeeper(strings.NewReader("not json"), agent.Polytoken, "/repo").Allow).To(BeTrue())
		payload := `{"tool_name":"file_read","input":{"path":".tickets/her-x.md"}}`
		Expect(runGatekeeper(strings.NewReader(payload), agent.Polytoken, "/repo").Allow).To(BeTrue())
	})

	It("fails open for an unknown harness", func() {
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		Expect(runGatekeeper(strings.NewReader(payload), agent.Name("unknown"), "/repo").Allow).To(BeTrue())
	})

	It("classifies but marks a relative Polytoken ticket unreadable when projectDir is empty", func() {
		path, readable := resolvePolytokenTicketPath(".tickets/her-empty-root.md", "")
		Expect(path).To(Equal(filepath.Join(string(filepath.Separator), ".tickets", "her-empty-root.md")))
		Expect(readable).To(BeFalse())
	})

	It("fails closed after recognizing a relative ticket when projectDir is empty", func() {
		payload := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-empty-root.md","new_string":"lifecycle: pending-validation\n"}}`
		d := runGatekeeper(strings.NewReader(payload), agent.Polytoken, "")
		Expect(d.Allow).To(BeFalse())
		Expect(d.Missing).To(Equal([]string{"standard-completed", "standard-addressed", "deep-completed", "deep-addressed"}))
	})

	It("keeps rollback and override semantics", func() {
		root := writeProject("validated", "")
		rollback := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n"}}`
		Expect(runGatekeeper(strings.NewReader(rollback), agent.Polytoken, root).Allow).To(BeTrue())

		root = writeProject("in-development", "")
		override := `{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: pending-validation\n[skip-code-review-gate] emergency\n"}}`
		Expect(runGatekeeper(strings.NewReader(override), agent.Polytoken, root).Allow).To(BeTrue())
	})
})
