package initcmd_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInitcmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Initcmd Suite")
}
