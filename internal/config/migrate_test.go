package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
)

var _ = Describe("MigrateWipProjects", func() {
	writeWip := func(body string) string {
		p := filepath.Join(GinkgoT().TempDir(), "projects")
		Expect(os.WriteFile(p, []byte(body), 0o600)).To(Succeed())
		return p
	}

	It("parses paths and gh= overrides, skipping comments and blanks", func() {
		p := writeWip("# header comment\n\n" +
			"/work/a            gh=owner/a\n" +
			"/work/b\n")
		got, err := config.MigrateWipProjects(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]config.Project{
			{Path: "/work/a", GH: "owner/a"},
			{Path: "/work/b"},
		}))
	})

	It("parses slug= overrides alongside gh=", func() {
		p := writeWip("/work/a slug=grp/proj\n/work/b gh=owner/b\n")
		got, err := config.MigrateWipProjects(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]config.Project{
			{Path: "/work/a", Slug: "grp/proj"},
			{Path: "/work/b", GH: "owner/b"},
		}))
	})

	It("returns an empty slice (no error) for a missing file", func() {
		got, err := config.MigrateWipProjects(filepath.Join(GinkgoT().TempDir(), "absent"))
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeEmpty())
	})

	It("merges into a config via Add without clobbering or duplicating", func() {
		p := writeWip("/work/a gh=owner/a\n/work/b\n")
		got, err := config.MigrateWipProjects(p)
		Expect(err).NotTo(HaveOccurred())

		c := &config.Config{Projects: []config.Project{{Path: "/work/a", Base: "dev"}}}
		for _, proj := range got {
			_ = c.Add(proj) // ignore ErrDuplicate
		}
		Expect(c.Projects).To(Equal([]config.Project{
			{Path: "/work/a", Base: "dev"}, // existing entry untouched
			{Path: "/work/b"},              // new entry appended
		}))
	})
})
