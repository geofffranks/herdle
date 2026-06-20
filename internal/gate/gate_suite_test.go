package gate_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gate Suite")
}
