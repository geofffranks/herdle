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

	"github.com/geofffranks/herdle/internal/gate"
)

var _ = Describe("hookCommand", func() {
	It("keeps code-review-gate as an alias of gatekeeper (upgrade migration window)", func() {
		var gk *cli.Command
		for _, c := range hookCommand().Subcommands {
			if c.Name == "gatekeeper" {
				gk = c
			}
		}
		Expect(gk).NotTo(BeNil())
		Expect(gk.Aliases).To(ContainElement("code-review-gate"))
	})
})

var _ = Describe("runGatekeeper", func() {
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
