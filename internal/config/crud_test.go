package config_test

import (
	"errors"

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
})
