package doctor_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDoctor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "doctor Suite")
}
