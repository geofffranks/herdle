package initcmd_test

import (
	"os"
	"path/filepath"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/initcmd"
)

// srcFS is a minimal stand-in for the embedded assets: one skill + one rule.
func srcFS() fstest.MapFS {
	return fstest.MapFS{
		"skills/herdle-tk-flow/SKILL.md": {Data: []byte("flow")},
		"rules/herdle.md":                {Data: []byte("rule")},
	}
}

var _ = Describe("Install", func() {
	It("writes the mirrored tree into claudeDir", func() {
		dir := GinkgoT().TempDir()
		results, err := initcmd.Install(srcFS(), dir, false)
		Expect(err).NotTo(HaveOccurred())

		skill := filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")
		Expect(skill).To(BeAnExistingFile())
		data, err := os.ReadFile(skill) // #nosec G304 -- test reads the file it just wrote
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("flow"))
		Expect(results).To(ContainElement(initcmd.Result{Path: skill, Action: initcmd.Written}))
		Expect(filepath.Join(dir, "rules", "herdle.md")).To(BeAnExistingFile())

		// S7 spec: installed artifacts are world-readable 0o644, not the 0o600
		// os.CreateTemp defaults to.
		info, err := os.Stat(skill)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o644)))
	})

	It("skips an existing file without force (preserving user edits)", func() {
		dir := GinkgoT().TempDir()
		skill := filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")
		Expect(os.MkdirAll(filepath.Dir(skill), 0o750)).To(Succeed())
		Expect(os.WriteFile(skill, []byte("user edit"), 0o600)).To(Succeed())

		results, err := initcmd.Install(srcFS(), dir, false)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(skill) // #nosec G304 -- test reads the file it just wrote
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("user edit")) // preserved
		Expect(results).To(ContainElement(initcmd.Result{Path: skill, Action: initcmd.Skipped}))
	})

	It("overwrites an existing file with force", func() {
		dir := GinkgoT().TempDir()
		skill := filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")
		Expect(os.MkdirAll(filepath.Dir(skill), 0o750)).To(Succeed())
		Expect(os.WriteFile(skill, []byte("user edit"), 0o600)).To(Succeed())

		results, err := initcmd.Install(srcFS(), dir, true)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(skill) // #nosec G304 -- test reads the file it just wrote
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("flow"))
		Expect(results).To(ContainElement(initcmd.Result{Path: skill, Action: initcmd.Overwritten}))
	})

	It("errors and leaves no temp file when the atomic rename fails", func() {
		dir := GinkgoT().TempDir()
		// A directory standing where a destination file should be makes the
		// file->dir rename inside writeAtomic fail, exercising its cleanup branch.
		collide := filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")
		Expect(os.MkdirAll(collide, 0o750)).To(Succeed())

		_, err := initcmd.Install(srcFS(), dir, true) // force -> attempts the write
		Expect(err).To(HaveOccurred())

		// No orphaned temp file is left beside the colliding destination.
		leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(collide), ".init-*.tmp"))
		Expect(err).NotTo(HaveOccurred())
		Expect(leftovers).To(BeEmpty())
	})
})

var _ = Describe("Uninstall", func() {
	It("removes shipped files, prunes empty dirs, and keeps foreign files", func() {
		dir := GinkgoT().TempDir()
		_, err := initcmd.Install(srcFS(), dir, false)
		Expect(err).NotTo(HaveOccurred())

		// a skill herdle did not ship
		foreign := filepath.Join(dir, "skills", "other", "x.md")
		Expect(os.MkdirAll(filepath.Dir(foreign), 0o750)).To(Succeed())
		Expect(os.WriteFile(foreign, []byte("mine"), 0o600)).To(Succeed())

		results, err := initcmd.Uninstall(srcFS(), dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(dir, "skills", "herdle-tk-flow")).NotTo(BeADirectory()) // pruned
		Expect(filepath.Join(dir, "rules")).NotTo(BeADirectory())                    // pruned
		Expect(foreign).To(BeAnExistingFile())                                       // foreign kept
		Expect(filepath.Join(dir, "skills")).To(BeADirectory())                      // kept (holds foreign)
		Expect(results).To(HaveLen(2))                                               // two shipped files removed
	})

	It("is a no-op when artifacts are already gone", func() {
		dir := GinkgoT().TempDir()
		results, err := initcmd.Uninstall(srcFS(), dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(BeEmpty())
	})
})
