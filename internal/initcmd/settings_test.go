package initcmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/initcmd"
)

var _ = Describe("MergeSettings / UnmergeSettings", func() {
	var path string
	BeforeEach(func() { path = filepath.Join(GinkgoT().TempDir(), "settings.json") })

	readMap := func() map[string]interface{} {
		b, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var m map[string]interface{}
		Expect(json.Unmarshal(b, &m)).To(Succeed())
		return m
	}

	It("creates settings.json with the gate entry when absent", func() {
		r, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Action).To(Equal(initcmd.Written))
		b, _ := os.ReadFile(path)
		Expect(string(b)).To(ContainSubstring("code-review-gate"))
		Expect(string(b)).To(ContainSubstring("Edit|Write|Bash"))
	})

	It("is idempotent — second merge does not duplicate", func() {
		_, _ = initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		r, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Action).To(Equal(initcmd.Skipped))
		hooks := readMap()["hooks"].(map[string]interface{})
		Expect(hooks["PreToolUse"].([]interface{})).To(HaveLen(1))
	})

	It("preserves unrelated settings and other PreToolUse hooks", func() {
		seed := map[string]interface{}{
			"theme": "dark",
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{"matcher": "Bash", "hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "~/.claude/other.sh"}}},
				},
			},
		}
		b, _ := json.MarshalIndent(seed, "", "  ")
		Expect(os.WriteFile(path, b, 0o600)).To(Succeed())

		_, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		m := readMap()
		Expect(m["theme"]).To(Equal("dark"))
		pre := m["hooks"].(map[string]interface{})["PreToolUse"].([]interface{})
		Expect(pre).To(HaveLen(2)) // existing + gate
	})

	It("updates the command when it changed (binary moved)", func() {
		_, _ = initcmd.MergeSettings(path, "/old/herdle hook code-review-gate")
		r, err := initcmd.MergeSettings(path, "/new/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Action).To(Equal(initcmd.Overwritten))
		b, _ := os.ReadFile(path)
		Expect(string(b)).To(ContainSubstring("/new/herdle"))
		Expect(string(b)).NotTo(ContainSubstring("/old/herdle"))
	})

	It("removes the gate entry on unmerge, leaving others", func() {
		_, _ = initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		r, err := initcmd.UnmergeSettings(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Action).To(Equal(initcmd.Removed))
		b, _ := os.ReadFile(path)
		Expect(string(b)).NotTo(ContainSubstring("code-review-gate"))
	})

	It("unmerge is a no-op when settings.json is absent", func() {
		r, err := initcmd.UnmergeSettings(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Action).To(Equal(initcmd.Skipped))
	})

	It("refuses to modify when hooks is not an object", func() {
		Expect(os.WriteFile(path, []byte(`{"hooks":"oops"}`), 0o600)).To(Succeed())
		_, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("hooks"))
		b, _ := os.ReadFile(path)
		Expect(string(b)).To(ContainSubstring(`"oops"`)) // original left intact, not clobbered
	})

	It("refuses to modify when PreToolUse is not an array", func() {
		Expect(os.WriteFile(path, []byte(`{"hooks":{"PreToolUse":{"x":1}}}`), 0o600)).To(Succeed())
		_, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("PreToolUse"))
	})

	It("creates a new settings.json with 0600 perms (secrets-bearing config)", func() {
		_, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		fi, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.Mode().Perm()).To(Equal(os.FileMode(0o600)))
	})

	It("preserves the existing file's permissions on merge", func() {
		Expect(os.WriteFile(path, []byte("{}"), 0o600)).To(Succeed())
		Expect(os.Chmod(path, 0o640)).To(Succeed())
		_, err := initcmd.MergeSettings(path, "/abs/herdle hook code-review-gate")
		Expect(err).NotTo(HaveOccurred())
		fi, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.Mode().Perm()).To(Equal(os.FileMode(0o640)))
	})
})
