package assets_test

import (
	"io/fs"
	"strings"
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

var _ = Describe("embedded final-integration review contract", func() {
	const cleanReviewContract = "## Herdle code review\n\n" +
		"- [x] Final integration review completed\n" +
		"- [x] Final integration review findings addressed\n" +
		"- [x] Final integration rereview not required\n"
	const completedRereviewContract = "## Herdle code review\n\n" +
		"- [x] Final integration review completed\n" +
		"- [x] Final integration review findings addressed\n" +
		"- [x] Final integration rereview completed\n" +
		"- [x] Final integration rereview findings addressed\n"

	readSkill := func(skillFS fs.FS, name string) string {
		content, err := fs.ReadFile(skillFS, "skills/"+name+"/SKILL.md")
		Expect(err).NotTo(HaveOccurred())
		return string(content)
	}

	semanticText := func(skill string) string {
		return strings.Join(strings.Fields(strings.ReplaceAll(skill, "**", "")), " ")
	}

	artifactSkills := func() []string {
		return []string{
			readSkill(assets.ClaudeFS, "herdle-tk-artifacts"),
			readSkill(assets.PolytokenFS, "herdle-tk-artifacts"),
		}
	}

	allReviewSkills := func() []string {
		return []string{
			readSkill(assets.ClaudeFS, "herdle-tk-artifacts"),
			readSkill(assets.PolytokenFS, "herdle-tk-artifacts"),
			readSkill(assets.ClaudeFS, "herdle-tk-flow"),
			readSkill(assets.PolytokenFS, "herdle-tk-flow"),
		}
	}

	It("keeps Claude and Polytoken artifact skills semantically aligned", func() {
		for _, skill := range artifactSkills() {
			semantics := semanticText(skill)
			Expect(semantics).To(ContainSubstring("one fresh final integration review"))
			Expect(semantics).To(ContainSubstring("one complete fixer batch"))
			Expect(semantics).To(ContainSubstring("only after branch-changing fixes"))
		}
	})

	It("emits both exact mutually exclusive new marker variants", func() {
		for _, skill := range artifactSkills() {
			Expect(skill).To(ContainSubstring(cleanReviewContract))
			Expect(skill).To(ContainSubstring(completedRereviewContract))
			Expect(skill).To(ContainSubstring("exactly one of these mutually exclusive forms"))
		}
	})

	It("removes Claude medium/high transcript proof", func() {
		for _, skillName := range []string{"herdle-tk-artifacts", "herdle-tk-flow"} {
			skill := readSkill(assets.ClaudeFS, skillName)
			Expect(skill).NotTo(ContainSubstring("/code-review"))
			Expect(skill).NotTo(ContainSubstring("medium"))
			Expect(skill).NotTo(ContainSubstring("high"))
			Expect(skill).NotTo(ContainSubstring("transcript"))
		}
	})

	It("keeps flow and artifact lifecycle gates aligned", func() {
		for _, skill := range allReviewSkills() {
			semantics := semanticText(skill)
			Expect(semantics).To(ContainSubstring("one fresh final integration review"))
			Expect(semantics).To(ContainSubstring("Critical and Important findings"))
			Expect(semantics).To(ContainSubstring("only after branch-changing fixes"))
		}
	})

	It("does not teach new documents to emit legacy markers", func() {
		legacyMarkers := []string{
			"Standard review completed",
			"Standard review findings addressed",
			"Deep review completed",
			"Deep review findings addressed",
		}
		for _, skill := range allReviewSkills() {
			for _, marker := range legacyMarkers {
				Expect(skill).NotTo(ContainSubstring(marker))
			}
		}
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
