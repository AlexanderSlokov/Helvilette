package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// othelaBase is the published port from e2e.compose.yml. Using the published port
// rather than a dynamically mapped one keeps the suite and a human at a terminal
// on the same URL.
const othelaBase = "http://127.0.0.1:8080"

// The fleet repository carries one manifest per subdirectory: fleet/nginx and
// fleet/baseline. Each points at its own playbook repository, which is the
// separation ADR-0003 introduced and which the previous single-repo fixture
// never exercised.
const bakedManifestCount = 2

// getJSON fetches a path off Othela and decodes it into out.
func getJSON(path string, out any) error {
	resp, err := http.Get(othelaBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// playbookNames returns the name of every playbook Othela has loaded.
func playbookNames() ([]string, error) {
	var playbooks []struct {
		Name string `json:"name"`
	}
	if err := getJSON("/api/v1/playbooks", &playbooks); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(playbooks))
	for _, pb := range playbooks {
		names = append(names, pb.Name)
	}
	return names, nil
}

// registeredNodes returns the node IDs Othela has seen register.
func registeredNodes() ([]string, error) {
	var nodes []struct {
		NodeID string `json:"node_id"`
	}
	if err := getJSON("/api/v1/nodes", &nodes); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.NodeID)
	}
	return ids, nil
}

var _ = Describe("Fleet sync", func() {
	It("loads every manifest in the fleet repository", func() {
		Eventually(playbookNames, 90*time.Second, 3*time.Second).
			Should(ConsistOf("nginx", "baseline"))
	})

	// Regression test for issue #32. Fleet sync used to resolve refs/heads/<branch>,
	// which a fetch never advances, so Othela re-checked-out its clone-time commit
	// on every poll and a manifest pushed after startup was never seen. See ADR-0005.
	It("picks up a manifest committed after startup", func() {
		before, err := playbookNames()
		Expect(err).NotTo(HaveOccurred())
		Expect(before).To(HaveLen(bakedManifestCount))

		manifest := strings.Join([]string{
			"apiVersion: helvilette.naughtian.org/v1alpha1",
			"kind: PlaybookDeployment",
			"metadata:",
			"  name: pushed-after-startup",
			"spec:",
			"  repo: git://git-server:9418/baseline",
			"  branch: main",
			"  playbook: playbook.yml",
			"  nodeGroups:",
			"    - name: nobody",
			"      nodeSelector:",
			"        role: unclaimed",
		}, "\n")

		sha, err := seedCommit("fleet", "late/helvilette.yml", manifest)
		Expect(err).NotTo(HaveOccurred())
		Expect(sha).To(MatchRegexp("^[0-9a-f]{40}$"))

		// One --fleet-sync-interval is 15s in the unit file; allow several.
		Eventually(playbookNames, 90*time.Second, 3*time.Second).
			Should(ContainElement("late"))

		By("logging the commit it moved to")
		logs, err := journal("othela", "helvilette-othela")
		Expect(err).NotTo(HaveOccurred())
		Expect(logs).To(ContainSubstring(sha), "othela did not report the commit it synced to")
		Expect(logs).To(ContainSubstring(`"message":"fleet updated"`))
	})
})

var _ = Describe("Label-based dispatch", func() {
	It("registers both agents", func() {
		Eventually(registeredNodes, 90*time.Second, 3*time.Second).
			Should(ConsistOf("node-1", "node-2"))
	})

	It("runs the edge-proxy playbook on node-1 and the baseline playbook on node-2", func() {
		// node-1 carries role=edge-proxy and node-2 role=baseline. The selectors
		// are disjoint on purpose: a node matching two manifests receives only the
		// first one scanned, which would make the outcome depend on walk order.
		// See ADR-0004.
		for _, node := range []string{"node-1", "node-2"} {
			Eventually(func() (string, error) { return journal(node, "helvilette-agent") },
				4*time.Minute, 5*time.Second).
				Should(ContainSubstring(`"message":"playbook execution succeeded"`),
					node+" never ran a playbook to completion")
		}

		By("reporting execution back to Othela")
		Eventually(func() (string, error) { return journal("othela", "helvilette-othela") },
			4*time.Minute, 5*time.Second).
			Should(ContainSubstring(`"message":"report received"`))
	})

	// ADR-0006 in the running stack: the loader must account for every path it
	// declined, not only report a count. Issue #34 was filed because a count of
	// zero arrived with nothing to act on.
	It("accounts for the paths the loader skipped", func() {
		logs, err := journal("othela", "helvilette-othela")
		Expect(err).NotTo(HaveOccurred())

		Expect(logs).To(ContainSubstring(`"skip_reason":"hidden_dir"`),
			"the fleet cache contains .git; the loader did not say it skipped it")
		Expect(logs).To(ContainSubstring(`"files_examined":`),
			"the scan summary did not say how many files it looked at")
	})
})

var _ = Describe("Units and journald", func() {
	// Helvilette manages systemd, so the e2e stack runs it under systemd. A
	// foreground-process container cannot exercise either this or the journal.
	// See ADR-0007 R2.
	It("runs Othela and the agents as active systemd units", func() {
		for service, unit := range map[string]string{
			"othela": "helvilette-othela.service",
			"node-1": "helvilette-agent.service",
			"node-2": "helvilette-agent.service",
		} {
			out, err := execIn(service, "systemctl", "is-active", unit)
			Expect(err).NotTo(HaveOccurred(), service+": "+out)
			Expect(strings.TrimSpace(out)).To(ContainSubstring("active"))
		}
	})

	// The shared SyslogIdentifier prefix is what lets an operator select the whole
	// product without knowing either unit name. It is also what a log collector
	// filters on, which is the point of ADR-0007 R1.
	It("exposes every component under a helvilette syslog identifier", func() {
		for service, identifier := range map[string]string{
			"othela": "helvilette-othela",
			"node-1": "helvilette-agent",
		} {
			out, err := journal(service, identifier)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(BeEmpty(), service+" wrote nothing under "+identifier)
			Expect(out).To(ContainSubstring(`"component":`), "journal entries are not Helvilette's structured logs")
		}
	})
})

var _ = Describe("Othela probes", func() {
	It("answers /healthz and /readyz", func() {
		for _, path := range []string{"/healthz", "/readyz"} {
			var body map[string]string
			Expect(getJSON(path, &body)).To(Succeed())
			Expect(body).To(HaveKeyWithValue("status", "ok"))
		}
	})
})
