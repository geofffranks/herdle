package agent_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/agent"
)

func TestAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Suite")
}

var _ = Describe("agent.Parse", func() {
	It("defaults to Claude", func() {
		got, err := agent.Parse(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]agent.Name{agent.Claude}))
	})
	It("preserves order while deduplicating", func() {
		got, err := agent.Parse([]string{"polytoken", "claude", "polytoken"})
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]agent.Name{agent.Polytoken, agent.Claude}))
	})
	It("rejects an unknown harness", func() {
		_, err := agent.Parse([]string{"cursor"})
		Expect(err).To(MatchError(`unknown agent "cursor" (expected claude or polytoken)`))
	})
})
