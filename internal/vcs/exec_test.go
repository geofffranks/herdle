package vcs

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("resolveBinary", func() {
	It("prefers the env override when set", func() {
		os.Setenv("HERDLE_TEST_BIN", "/custom/path/tool")
		DeferCleanup(func() { os.Unsetenv("HERDLE_TEST_BIN") })

		got, err := resolveBinary("HERDLE_TEST_BIN", "definitely-not-on-path")

		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal("/custom/path/tool"))
	})

	It("falls back to PATH lookup when unset", func() {
		os.Unsetenv("HERDLE_TEST_BIN")

		got, err := resolveBinary("HERDLE_TEST_BIN", "go")

		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(ContainSubstring("go"))
	})
})

var _ = Describe("run", func() {
	writeStub := func(dir, name, body string) string {
		p := filepath.Join(dir, name)
		Expect(os.WriteFile(p, []byte(body), 0o755)).To(Succeed()) // #nosec G306 -- stub must be executable
		return p
	}

	It("captures stdout and a zero exit", func() {
		dir := GinkgoT().TempDir()
		stub := writeStub(dir, "echoer", "#!/bin/sh\necho hello\n")

		res, err := run(dir, stub)

		Expect(err).NotTo(HaveOccurred())
		Expect(res.code).To(Equal(0))
		Expect(res.trimmed()).To(Equal("hello"))
	})

	It("reports a non-zero exit without returning an error", func() {
		dir := GinkgoT().TempDir()
		stub := writeStub(dir, "failer", "#!/bin/sh\nexit 3\n")

		res, err := run(dir, stub)

		Expect(err).NotTo(HaveOccurred())
		Expect(res.code).To(Equal(3))
	})

	It("returns an error when the binary cannot be started", func() {
		_, err := run("", "/nonexistent/binary-xyz")
		Expect(err).To(HaveOccurred())
	})
})
