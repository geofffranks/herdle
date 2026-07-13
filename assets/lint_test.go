package assets_test

import (
	"io/fs"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/assets"
)

var _ = Describe("embedded skill artifacts", func() {
	It("lints both harness trees", func() {
		Expect(lintSkills(assets.ClaudeFS, "rules/herdle.md")).To(BeEmpty())
		Expect(lintSkills(assets.PolytokenFS, "herdle.md")).To(BeEmpty())
	})

	It("keeps Polytoken assets harness-native", func() {
		err := fs.WalkDir(assets.PolytokenFS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, readErr := fs.ReadFile(assets.PolytokenFS, p)
			Expect(readErr).NotTo(HaveOccurred())
			text := string(b)
			Expect(text).NotTo(ContainSubstring("CLAUDE.md"), p)
			Expect(text).NotTo(ContainSubstring("TodoWrite"), p)
			Expect(text).NotTo(ContainSubstring("/code-review"), p)
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("lintSkills", func() {
	good := func() fstest.MapFS {
		return fstest.MapFS{
			"skills/foo/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: foo\ndescription: Use when foo.\n---\nbody\n")},
			"rules/herdle.md":     &fstest.MapFile{Data: []byte("herdle orientation line.\n")},
		}
	}

	It("returns no problems for a well-formed tree", func() {
		Expect(lintSkills(good(), "rules/herdle.md")).To(BeEmpty())
	})

	It("flags a name/directory mismatch", func() {
		m := good()
		m["skills/foo/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: bar\ndescription: x\n---\n")}
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("does not match directory foo")))
	})

	It("flags a missing description", func() {
		m := good()
		m["skills/foo/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: foo\n---\n")}
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("empty or missing description")))
	})

	It("flags a skill directory with no SKILL.md", func() {
		m := good()
		delete(m, "skills/foo/SKILL.md")
		m["skills/foo/other.md"] = &fstest.MapFile{Data: []byte("x")}
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("missing SKILL.md")))
	})

	It("flags malformed frontmatter", func() {
		m := good()
		m["skills/foo/SKILL.md"] = &fstest.MapFile{Data: []byte("no frontmatter here\n")}
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("malformed frontmatter")))
	})

	It("flags a rules file with a paths: key", func() {
		m := good()
		m["rules/herdle.md"] = &fstest.MapFile{Data: []byte("---\npaths: src/**\n---\nbody\n")}
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("paths: key")))
	})

	It("flags a missing rules file", func() {
		m := good()
		delete(m, "rules/herdle.md")
		Expect(lintSkills(m, "rules/herdle.md")).To(ContainElement(ContainSubstring("rules/herdle.md: missing")))
	})
})
