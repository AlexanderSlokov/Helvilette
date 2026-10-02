package e2e_test

import (
	"os"
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

// Diagnostics are gathered here, not in a CI step, because AfterSuite tears the
// stack down: a step that shells out to docker afterwards finds no containers
// and prints nothing. The first version of this suite did exactly that, and a
// failing playbook on CI reported only "exit status 2".
var _ = AfterEach(func() {
	if !CurrentSpecReport().Failed() {
		return
	}
	AddReportEntry("stack diagnostics", diagnose())
})

var _ = AfterSuite(func() {
	// HELV_KEEP_STACK leaves the containers running so a failure can be opened
	// up by hand. Never set in CI, where leaked containers outlive the job.
	if os.Getenv("HELV_KEEP_STACK") != "" {
		GinkgoWriter.Println("HELV_KEEP_STACK set; leaving the stack up. Tear down with `make down`.")
		return
	}

	out, err := composeDown()
	Expect(err).NotTo(HaveOccurred(), out)
})
