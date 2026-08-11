package config_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
)

var _ = Describe("config CRUD", func() {
	It("adds projects and rejects a duplicate path", func() {
		c := &config.Config{}
		Expect(c.Add(config.Project{Path: "/work/a"})).To(Succeed())
		Expect(c.Add(config.Project{Path: "/work/b"})).To(Succeed())
		Expect(c.Projects).To(HaveLen(2))

		err := c.Add(config.Project{Path: "/work/a"})
		Expect(errors.Is(err, config.ErrDuplicate)).To(BeTrue())
		Expect(c.Projects).To(HaveLen(2)) // unchanged
	})

	It("finds by exact path and by basename", func() {
		c := &config.Config{Projects: []config.Project{
			{Path: "/work/a"}, {Path: "/other/b"},
		}}
		i, err := c.Find("/work/a")
		Expect(err).NotTo(HaveOccurred())
		Expect(i).To(Equal(0))

		i, err = c.Find("b") // basename
		Expect(err).NotTo(HaveOccurred())
		Expect(i).To(Equal(1))
	})

	It("returns ErrNotFound for an unknown key", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/work/a"}}}
		_, err := c.Find("nope")
		Expect(errors.Is(err, config.ErrNotFound)).To(BeTrue())
	})

	It("reports an ambiguous basename, listing candidate paths", func() {
		c := &config.Config{Projects: []config.Project{
			{Path: "/x/config"}, {Path: "/y/config"},
		}}
		_, err := c.Find("config")
		var amb *config.AmbiguousError
		Expect(errors.As(err, &amb)).To(BeTrue())
		Expect(amb.Paths).To(ConsistOf("/x/config", "/y/config"))
		// the exact path is still unambiguous
		i, err := c.Find("/y/config")
		Expect(err).NotTo(HaveOccurred())
		Expect(i).To(Equal(1))
	})

	It("removes by index", func() {
		c := &config.Config{Projects: []config.Project{
			{Path: "/a"}, {Path: "/b"}, {Path: "/c"},
		}}
		c.Remove(1)
		Expect(c.Projects).To(Equal([]config.Project{{Path: "/a"}, {Path: "/c"}}))
	})

	It("upserts project Polytoken state by exact canonical path", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/work/app", Slug: "owner/app"}}}
		Expect(c.UpsertProjectPolytoken("/work/app")).To(BeTrue())
		Expect(c.Projects).To(Equal([]config.Project{{Path: "/work/app", Slug: "owner/app", Polytoken: true}}))
		Expect(c.UpsertProjectPolytoken("/work/app")).To(BeFalse())

		Expect(c.UpsertProjectPolytoken("/work/other")).To(BeTrue())
		Expect(c.Projects[1]).To(Equal(config.Project{Path: "/work/other", Polytoken: true}))
	})

	It("does not use basename lookup when upserting project Polytoken state", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/one/app", Slug: "owner/one"}}}
		Expect(c.UpsertProjectPolytoken("/two/app")).To(BeTrue())
		Expect(c.Projects).To(Equal([]config.Project{
			{Path: "/one/app", Slug: "owner/one"},
			{Path: "/two/app", Polytoken: true},
		}))
	})

	It("clears project Polytoken state while preserving other metadata", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/work/app", Slug: "owner/app", Polytoken: true}}}
		Expect(c.ClearProjectPolytoken("/work/app")).To(BeTrue())
		Expect(c.Projects).To(Equal([]config.Project{{Path: "/work/app", Slug: "owner/app"}}))
		Expect(c.ClearProjectPolytoken("/work/app")).To(BeFalse())
	})

	It("removes a path-only project after clearing Polytoken state", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/work/app", Polytoken: true}}}
		Expect(c.ClearProjectPolytoken("/work/app")).To(BeTrue())
		Expect(c.Projects).To(BeEmpty())
		Expect(c.ClearProjectPolytoken("/work/app")).To(BeFalse())
	})

	It("keeps basename Find behavior after exact project installation mutations", func() {
		c := &config.Config{Projects: []config.Project{{Path: "/work/app"}}}
		Expect(c.UpsertProjectPolytoken("/work/app")).To(BeTrue())
		idx, err := c.Find("app")
		Expect(err).NotTo(HaveOccurred())
		Expect(idx).To(Equal(0))
	})
})

var _ = Describe("project installation mutation paths", func() {
	It("canonicalizes dot, dot-dot, separators, and symlink aliases", func() {
		physical := GinkgoT().TempDir()
		project := filepath.Join(physical, "project")
		Expect(os.Mkdir(project, 0o750)).To(Succeed())
		alias := filepath.Join(GinkgoT().TempDir(), "alias")
		Expect(os.Symlink(project, alias)).To(Succeed())

		got, err := config.CanonicalProjectPath(filepath.Join(alias, ".", "child", "..") + string(os.PathSeparator))
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(project))
	})

	It("fails when the full path cannot be physically resolved", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "missing")
		_, err := config.CanonicalProjectPath(missing)
		Expect(err).To(HaveOccurred())
	})
})
