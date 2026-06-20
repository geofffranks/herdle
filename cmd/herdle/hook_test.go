package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("runCodeReviewGate", func() {
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

	It("allows when both passes are present", func() {
		tp := writeTranscript(skill("medium"), skill("high"))
		ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
		Expect(runCodeReviewGate(stdin(ti, tp)).Allow).To(BeTrue())
	})
	It("blocks when a pass is missing", func() {
		tp := writeTranscript(skill("medium"))
		ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
		Expect(runCodeReviewGate(stdin(ti, tp)).Allow).To(BeFalse())
	})
	It("fails closed when the transcript path is missing", func() {
		ti := `{"file_path":"/repo/.tickets/her-5s12.md","new_string":"lifecycle: pending-validation\n"}`
		Expect(runCodeReviewGate(stdin(ti, "/no/such/transcript.jsonl")).Allow).To(BeFalse())
	})
	It("allows on malformed stdin (fail-open on envelope parse)", func() {
		Expect(runCodeReviewGate(strings.NewReader("not json")).Allow).To(BeTrue())
	})
})
