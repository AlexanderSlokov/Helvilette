package e2e_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

// The stack is brought up once for the whole suite, not per spec. Each spec used
// to build four images and boot them from scratch, which cost minutes per
// assertion and made the suite slow enough that its failures went unread — the
// stale assertion that broke it in September 2026 sat unnoticed for three weeks.
var _ = BeforeSuite(func() {
	out, err := composeUp()
	if err != nil {
		AbortSuite("could not bring up " + composeFile + ": " + err.Error() + "\n" + out)
	}
})

var _ = AfterSuite(func() {
	out, err := composeDown()
	Expect(err).NotTo(HaveOccurred(), out)
})
