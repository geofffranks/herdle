package initcmd_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/initcmd"
)

const gatekeeperCommand = "/bin/herdle hook gatekeeper --agent polytoken"

func polytokenFS() fstest.MapFS {
	return fstest.MapFS{
		"skills/herdle-tk-flow/SKILL.md":       {Data: []byte("flow")},
		"skills/herdle-using-tickets/SKILL.md": {Data: []byte("tickets")},
		"herdle.md":                            {Data: []byte("context")},
	}
}

func readFile(path string) string {
	data, err := os.ReadFile(path) // #nosec G304 -- tests read their own temp files
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return string(data)
}

func readHooks(path string) []map[string]any {
	var hooks []map[string]any
	ExpectWithOffset(1, json.Unmarshal([]byte(readFile(path)), &hooks)).To(Succeed())
	return hooks
}

func fileMode(path string) os.FileMode {
	info, err := os.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return info.Mode().Perm()
}

var _ = Describe("Polytoken", func() {
	It("installs standalone assets, a gatekeeper hook, and managed context", func() {
		dir := GinkgoT().TempDir()
		results, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())

		skill := filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")
		Expect(skill).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "herdle.md")).To(BeAnExistingFile())
		Expect(readHooks(filepath.Join(dir, "hooks.json"))).To(ContainElement(And(
			HaveKeyWithValue("name", "herdle-gatekeeper"),
			HaveKeyWithValue("event", "pre_tool_use"),
			HaveKeyWithValue("matcher", "*"),
		)))
		Expect(readFile(filepath.Join(dir, "AGENTS.md"))).To(Equal("<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"))
		Expect(fileMode(filepath.Join(dir, "hooks.json"))).To(Equal(os.FileMode(0o600)))
		Expect(fileMode(filepath.Join(dir, "AGENTS.md"))).To(Equal(os.FileMode(0o644)))
		Expect(fileMode(filepath.Join(dir, "herdle.md"))).To(Equal(os.FileMode(0o644)))
		Expect(results).To(ContainElement(initcmd.Result{Path: skill, Action: initcmd.Written}))
	})

	It("preserves foreign hooks and all pre-existing markdown bytes across install and uninstall", func() {
		dir := GinkgoT().TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")
		agentsPath := filepath.Join(dir, "AGENTS.md")
		foreignHooks := "[\n  {\"name\":\"first\",\"custom\": {\"x\":1}},\n  {\"name\":\"middle\",\"values\":[\"foreign-array\"]},\n  {\"name\":\"last\",\"enabled\":true}\n]\n"
		foreignMarkdown := "# Mine\n\nKeep  two spaces  \n"
		Expect(os.WriteFile(hooksPath, []byte(foreignHooks), 0o640)).To(Succeed())
		Expect(os.WriteFile(agentsPath, []byte(foreignMarkdown), 0o600)).To(Succeed())

		_, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(fileMode(hooksPath)).To(Equal(os.FileMode(0o640)))
		Expect(fileMode(agentsPath)).To(Equal(os.FileMode(0o600)))
		Expect(readFile(agentsPath)).To(Equal(foreignMarkdown + "\n<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"))
		hooks := readHooks(hooksPath)
		Expect(hooks).To(HaveLen(4))
		Expect(hooks[0]).To(HaveKeyWithValue("name", "first"))
		Expect(hooks[2]).To(HaveKeyWithValue("name", "last"))

		_, err = initcmd.UninstallPolytoken(polytokenFS(), dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(readFile(agentsPath)).To(Equal(foreignMarkdown + "\n"))
		Expect(readHooks(hooksPath)).To(Equal([]map[string]any{
			{"name": "first", "custom": map[string]any{"x": float64(1)}},
			{"name": "middle", "values": []any{"foreign-array"}},
			{"name": "last", "enabled": true},
		}))
	})

	It("reports Merged when appending into foreign files and Overwritten when refreshing managed content", func() {
		dir := GinkgoT().TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")
		agentsPath := filepath.Join(dir, "AGENTS.md")
		// Foreign content, no Herdle-managed hook/block yet.
		Expect(os.WriteFile(hooksPath, []byte("[{\"name\":\"other\",\"event\":\"session_start\"}]\n"), 0o600)).To(Succeed())
		Expect(os.WriteFile(agentsPath, []byte("# My notes\n"), 0o644)).To(Succeed())

		results, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(ContainElement(initcmd.Result{Path: hooksPath, Action: initcmd.Merged}))
		Expect(results).To(ContainElement(initcmd.Result{Path: agentsPath, Action: initcmd.Merged}))

		// Re-running refreshes the now-managed hook/block and reports Overwritten.
		results2, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(results2).To(ContainElement(initcmd.Result{Path: hooksPath, Action: initcmd.Overwritten}))
		Expect(results2).To(ContainElement(initcmd.Result{Path: agentsPath, Action: initcmd.Overwritten}))
	})

	It("inspects the exact installed hook and context through the shared parsers", func() {
		dir := GinkgoT().TempDir()
		_, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())

		hook, err := initcmd.InspectPolytokenHooks(filepath.Join(dir, "hooks.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(hook.Count).To(Equal(1))
		Expect(hook.Event).To(Equal("pre_tool_use"))
		Expect(hook.Matcher).To(Equal("*"))
		Expect(hook.Command).To(Equal(gatekeeperCommand))
		context, err := initcmd.InspectAgentContext(filepath.Join(dir, "AGENTS.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(context.Count).To(Equal(1))
		Expect(context.Exact).To(BeTrue())
	})

	It("self-heals a stale managed hook and context without force", func() {
		dir := GinkgoT().TempDir()
		hooksPath := filepath.Join(dir, "hooks.json")
		agentsPath := filepath.Join(dir, "AGENTS.md")
		Expect(os.WriteFile(hooksPath, []byte(`[{"name":"herdle-gatekeeper","event":"old","matcher":"old","handler":{"bash":"old"}}]`), 0o600)).To(Succeed())
		Expect(os.WriteFile(agentsPath, []byte("before\n<!-- herdle:begin -->\nstale\n<!-- herdle:end -->\nafter\n"), 0o644)).To(Succeed())

		_, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		inspection, err := initcmd.InspectPolytokenHooks(hooksPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(inspection.Command).To(Equal(gatekeeperCommand))
		Expect(readFile(agentsPath)).To(Equal("before\n<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\nafter\n"))
	})

	It("skips standalone files without force and overwrites them with force", func() {
		dir := GinkgoT().TempDir()
		herdleDoc := filepath.Join(dir, "herdle.md")
		Expect(os.WriteFile(herdleDoc, []byte("mine"), 0o600)).To(Succeed())
		results, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(readFile(herdleDoc)).To(Equal("mine"))
		Expect(results).To(ContainElement(initcmd.Result{Path: herdleDoc, Action: initcmd.Skipped}))

		results, err = initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(readFile(herdleDoc)).To(Equal("context"))
		Expect(results).To(ContainElement(initcmd.Result{Path: herdleDoc, Action: initcmd.Overwritten}))
	})

	It("selects exactly the three Polytoken-owned standalone files", func() {
		dir := GinkgoT().TempDir()
		src := polytokenFS()
		src["skills/herdle-tk-artifacts/SKILL.md"] = &fstest.MapFile{Data: []byte("artifacts")}
		src["skills/herdle-tk-flow/user-notes.md"] = &fstest.MapFile{Data: []byte("source flow notes")}
		src["skills/herdle-tk-artifacts/foreign.json"] = &fstest.MapFile{Data: []byte("source artifact data")}
		src["foreign/asset.txt"] = &fstest.MapFile{Data: []byte("source foreign")}
		flowNotes := filepath.Join(dir, "skills", "herdle-tk-flow", "user-notes.md")
		artifactData := filepath.Join(dir, "skills", "herdle-tk-artifacts", "foreign.json")
		foreignPath := filepath.Join(dir, "foreign", "asset.txt")

		_, err := initcmd.InstallPolytoken(src, dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "skills", "herdle-tk-artifacts", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "herdle.md")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "skills", "herdle-using-tickets", "SKILL.md")).NotTo(BeAnExistingFile())
		Expect(flowNotes).NotTo(BeAnExistingFile())
		Expect(artifactData).NotTo(BeAnExistingFile())
		Expect(foreignPath).NotTo(BeAnExistingFile())

		Expect(os.WriteFile(flowNotes, []byte("destination flow notes"), 0o640)).To(Succeed())
		Expect(os.WriteFile(artifactData, []byte("destination artifact data"), 0o640)).To(Succeed())
		Expect(os.MkdirAll(filepath.Dir(foreignPath), 0o750)).To(Succeed())
		Expect(os.WriteFile(foreignPath, []byte("destination foreign"), 0o640)).To(Succeed())
		_, err = initcmd.UninstallPolytoken(src, dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(readFile(flowNotes)).To(Equal("destination flow notes"))
		Expect(readFile(artifactData)).To(Equal("destination artifact data"))
		Expect(readFile(foreignPath)).To(Equal("destination foreign"))
	})

	It("selects every regular file embedded in assets.PolytokenFS (invariant guard)", func() {
		// If a future asset is added under assets/polytoken without a matching
		// isPolytokenStandalone entry, InstallPolytoken would silently skip it
		// (it is never written to disk) while doctor reports the install as
		// incomplete. This guard fails the build before that drift ships.
		var unselected []string
		err := fs.WalkDir(assets.PolytokenFS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !initcmd.IsPolytokenStandalone(p) {
				unselected = append(unselected, p)
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(unselected).To(BeEmpty(), "assets.PolytokenFS files not selected by isPolytokenStandalone: %v", unselected)
	})

	It("prunes only directories that contain selected Polytoken files", func() {
		dir := GinkgoT().TempDir()
		src := polytokenFS()
		src["skills/herdle-tk-artifacts/SKILL.md"] = &fstest.MapFile{Data: []byte("artifacts")}
		src["foreign/tree/asset.txt"] = &fstest.MapFile{Data: []byte("source foreign")}
		foreignDir := filepath.Join(dir, "foreign", "tree")

		_, err := initcmd.InstallPolytoken(src, dir, gatekeeperCommand, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(foreignDir, 0o750)).To(Succeed())

		_, err = initcmd.UninstallPolytoken(src, dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(dir, "skills", "herdle-tk-flow")).NotTo(BeADirectory())
		Expect(filepath.Join(dir, "skills", "herdle-tk-artifacts")).NotTo(BeADirectory())
		Expect(filepath.Join(dir, "skills")).NotTo(BeADirectory())
		Expect(foreignDir).To(BeADirectory())
	})

	DescribeTable("preserves an existing standalone file mode when force overwrites it",
		func(mode os.FileMode) {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "herdle.md")
			Expect(os.WriteFile(path, []byte("old"), 0o600)).To(Succeed())
			Expect(os.Chmod(path, mode)).To(Succeed())

			results, err := initcmd.Install(fstest.MapFS{"herdle.md": {Data: []byte("new")}}, dir, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(ContainElement(initcmd.Result{Path: path, Action: initcmd.Overwritten}))
			Expect(fileMode(path)).To(Equal(mode))
		},
		Entry("0600", os.FileMode(0o600)),
		Entry("0640", os.FileMode(0o640)),
		Entry("0000", os.FileMode(0o000)),
	)

	DescribeTable("reports an existing empty AGENTS.md as merged and preserves its mode",
		func(mode os.FileMode) {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "AGENTS.md")
			Expect(os.WriteFile(path, nil, 0o600)).To(Succeed())
			Expect(os.Chmod(path, mode)).To(Succeed())

			results, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(ContainElement(initcmd.Result{Path: path, Action: initcmd.Merged}))
			Expect(fileMode(path)).To(Equal(mode))
		},
		Entry("0640", os.FileMode(0o640)),
	)

	It("unmerges only the managed context range", func() {
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "AGENTS.md")
		contents := "foreign before\n<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\nforeign after\n"
		Expect(os.WriteFile(path, []byte(contents), 0o640)).To(Succeed())

		_, err := initcmd.UninstallPolytoken(polytokenFS(), dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(readFile(path)).To(Equal("foreign before\nforeign after\n"))
	})

	It("treats a CRLF AGENTS.md managed block as exact (Windows compatibility)", func() {
		// On Windows a clean install writes LF, but editors/tools may re-encode
		// the file to CRLF. The captured bytes then differ from the LF-only
		// contextBlock constant, which made doctor false-positive "malformed".
		// parseAgentContext must normalize CR before the exact comparison.
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "AGENTS.md")
		// Build a CRLF file: a leading line, then the managed block with CRLF.
		body := "intro\r\n<!-- herdle:begin -->\r\n@herdle.md\r\n<!-- herdle:end -->\r\n"
		Expect(os.WriteFile(path, []byte(body), 0o644)).To(Succeed())

		context, err := initcmd.InspectAgentContext(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(context.Count).To(Equal(1))
		Expect(context.Exact).To(BeTrue())
	})

	DescribeTable("rejects ambiguous hooks without modifying them",
		func(contents string) {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "hooks.json")
			Expect(os.WriteFile(path, []byte(contents), 0o640)).To(Succeed())
			_, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
			Expect(err).To(MatchError(ContainSubstring(path)))
			Expect(readFile(path)).To(Equal(contents))
			_, inspectErr := initcmd.InspectPolytokenHooks(path)
			Expect(inspectErr).To(MatchError(ContainSubstring(path)))
		},
		Entry("malformed JSON", "["),
		Entry("object top level", `{}`),
		Entry("duplicate managed names", `[{"name":"herdle-gatekeeper"},{"name":"herdle-gatekeeper"}]`),
	)

	DescribeTable("rejects ambiguous markers without modifying them",
		func(contents string) {
			dir := GinkgoT().TempDir()
			path := filepath.Join(dir, "AGENTS.md")
			Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
			_, err := initcmd.InstallPolytoken(polytokenFS(), dir, gatekeeperCommand, false)
			Expect(err).To(MatchError(ContainSubstring(path)))
			Expect(readFile(path)).To(Equal(contents))
			_, inspectErr := initcmd.InspectAgentContext(path)
			Expect(inspectErr).To(MatchError(ContainSubstring(path)))
		},
		Entry("begin only", "<!-- herdle:begin -->\n"),
		Entry("end only", "<!-- herdle:end -->\n"),
		Entry("reversed", "<!-- herdle:end -->\n<!-- herdle:begin -->\n"),
		Entry("duplicate begin", "<!-- herdle:begin -->\n<!-- herdle:begin -->\n<!-- herdle:end -->\n"),
		Entry("duplicate end", "<!-- herdle:begin -->\n<!-- herdle:end -->\n<!-- herdle:end -->\n"),
		Entry("nested", "<!-- herdle:begin -->\n<!-- herdle:begin -->\n<!-- herdle:end -->\n<!-- herdle:end -->\n"),
	)

	It("reports absent files as zero-count inspections and uninstalls idempotently", func() {
		dir := GinkgoT().TempDir()
		hook, err := initcmd.InspectPolytokenHooks(filepath.Join(dir, "hooks.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(hook.Count).To(BeZero())
		context, err := initcmd.InspectAgentContext(filepath.Join(dir, "AGENTS.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(context.Count).To(BeZero())

		results, err := initcmd.UninstallPolytoken(polytokenFS(), dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(BeEmpty())
	})
})
