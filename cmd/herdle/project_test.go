package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/config"
)

var _ = Describe("herdle project", func() {
	var (
		buf, errBuf *bytes.Buffer
		app         *cli.App
		cfgPath     string
		repo        string
	)

	run := func(args ...string) error {
		return app.Run(append([]string{"herdle"}, args...))
	}

	BeforeEach(func() {
		buf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
		app = newApp()
		app.Writer = buf
		app.ErrWriter = errBuf

		cfgPath = filepath.Join(GinkgoT().TempDir(), "config.toml")
		os.Setenv("HERDLE_CONFIG", cfgPath)
		DeferCleanup(func() { os.Unsetenv("HERDLE_CONFIG") })

		// A real on-disk directory so `add` passes its existence check.
		repo = GinkgoT().TempDir()
	})

	It("adds a project and lists it", func() {
		Expect(run("project", "add", repo, "--base", "dev")).To(Succeed())
		c, err := config.LoadFrom(cfgPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Projects).To(HaveLen(1))
		Expect(c.Projects[0].Path).To(Equal(repo))
		Expect(c.Projects[0].Base).To(Equal("dev"))

		buf.Reset()
		Expect(run("project", "list")).To(Succeed())
		Expect(buf.String()).To(ContainSubstring(filepath.Base(repo)))
		Expect(buf.String()).To(ContainSubstring("dev"))
	})

	It("rejects a duplicate and a non-existent path", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		Expect(run("project", "add", repo)).To(MatchError(ContainSubstring("already configured")))
		Expect(run("project", "add", "/no/such/dir")).To(HaveOccurred())
	})

	It("sets a field and clears it with an empty value", func() {
		Expect(run("project", "add", repo, "--base", "dev")).To(Succeed())
		Expect(run("project", "set", filepath.Base(repo), "--base", "")).To(Succeed())
		c, _ := config.LoadFrom(cfgPath)
		Expect(c.Projects[0].Base).To(Equal("")) // cleared -> re-sparsed
	})

	It("treats a valueless flag before another flag as a clear, not a swallow", func() {
		Expect(run("project", "add", repo, "--base", "dev")).To(Succeed())
		// `--base` has no value (next token is `--remote`), so base is cleared
		// while `--remote main` is still parsed correctly.
		Expect(run("project", "set", filepath.Base(repo), "--base", "--remote", "main")).To(Succeed())
		c, _ := config.LoadFrom(cfgPath)
		Expect(c.Projects[0].Base).To(Equal(""))       // valueless --base cleared it
		Expect(c.Projects[0].Remote).To(Equal("main")) // --remote main not consumed by --base
	})

	It("removes a project", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		Expect(run("project", "rm", filepath.Base(repo))).To(Succeed())
		c, _ := config.LoadFrom(cfgPath)
		Expect(c.Projects).To(BeEmpty())
	})

	It("prints a friendly message when listing an empty config", func() {
		Expect(run("project", "list")).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("no projects configured"))
	})

	It("accepts -- end-of-options before the path", func() {
		Expect(run("project", "add", "--", repo)).To(Succeed())
		c, err := config.LoadFrom(cfgPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Projects).To(HaveLen(1))
		Expect(c.Projects[0].Path).To(Equal(repo))
	})

	It("rejects an unknown flag on add", func() {
		Expect(run("project", "add", repo, "--bogus", "x")).To(MatchError(ContainSubstring("unknown flag")))
	})

	It("rejects an unknown flag on set", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		Expect(run("project", "set", filepath.Base(repo), "--nope", "y")).To(MatchError(ContainSubstring("unknown flag")))
	})

	It("errors when set is called with no flags (no-op guard)", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		err := run("project", "set", filepath.Base(repo))
		Expect(err).To(MatchError(ContainSubstring("no fields to update")))
	})

	It("errors when add receives more than one positional argument", func() {
		other := GinkgoT().TempDir()
		err := run("project", "add", repo, other)
		Expect(err).To(MatchError(ContainSubstring("exactly one <path> argument is required")))
	})

	It("errors when set receives more than one positional argument", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		other := GinkgoT().TempDir()
		err := run("project", "set", repo, other, "--base", "dev")
		Expect(err).To(MatchError(ContainSubstring("exactly one <name|path> argument is required")))
	})

	It("set finds the project by its absolute path", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		// Use the absolute path (repo is already absolute from TempDir).
		Expect(run("project", "set", repo, "--base", "dev")).To(Succeed())
		c, _ := config.LoadFrom(cfgPath)
		Expect(c.Projects[0].Base).To(Equal("dev"))
	})

	It("rm finds the project by its absolute path", func() {
		Expect(run("project", "add", repo)).To(Succeed())
		Expect(run("project", "rm", repo)).To(Succeed())
		c, _ := config.LoadFrom(cfgPath)
		Expect(c.Projects).To(BeEmpty())
	})
})
