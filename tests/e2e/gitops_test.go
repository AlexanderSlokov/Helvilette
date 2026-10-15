package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// othelaBase is where Othela is published by e2e.compose.yml. Using the
// published port rather than a dynamically mapped one keeps the suite and a
// human at a terminal on the same URL. HELV_OTHELA_PORT overrides it on both
// sides at once, because 8080 is not free on every machine.
func othelaBase() string {
	port := os.Getenv("HELV_OTHELA_PORT")
	if port == "" {
		port = "8080"
	}
	return "http://127.0.0.1:" + port
}

// The fleet repository carries one manifest per subdirectory: fleet/nginx and
// fleet/baseline. Each points at its own playbook repository, which is the
// separation ADR-0003 introduced and which the previous single-repo fixture
// never exercised.
const bakedManifestCount = 2

// getJSON fetches a path off Othela and decodes it into out.
func getJSON(path string, out any) error {
	resp, err := http.Get(othelaBase() + path)
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

// agentStatusPath is the agent's own record of its last run. Since ADR-0008 it
// replaces the report Othela used to receive.
const agentStatusPath = "/var/lib/helvilette/agent/last_run_summary.json"

// agentRunStatus is the part of last_run_summary.json the specs assert on.
type agentRunStatus struct {
	RunID     string `json:"job_id"`
	CommitSHA string `json:"commit_sha"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

// lastRun reads a node's last_run_summary.json.
func lastRun(node string) (agentRunStatus, error) {
	var status agentRunStatus
	out, err := execIn(node, "cat", agentStatusPath)
	if err != nil {
		return status, err
	}
	err = json.Unmarshal([]byte(out), &status)
	return status, err
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
		Expect(logs).To(ContainSubstring(`"MESSAGE":"fleet updated"`))
	})
})

// The agent pulls its own repository and needs no Othela (ADR-0008 D1). node-1
// carries role=edge-proxy and pulls nginx-collection, whose root helvilette.yml
// (v1alpha2, ADR-0009) selects the nginx playbook with extra_vars.
var _ = Describe("Standalone agent", func() {
	It("runs the playbook its labels select, from its own repository", func() {
		for _, node := range agentNodes {
			Eventually(func() (string, error) { return journal(node, "helvilette-agent") },
				4*time.Minute, 5*time.Second).
				Should(ContainSubstring(`"MESSAGE":"playbook execution succeeded"`),
					node+" never ran a playbook to completion")

			Eventually(func() (agentRunStatus, error) { return lastRun(node) }, 4*time.Minute, 5*time.Second).
				Should(HaveField("Status", "Success"), node+" did not record a successful run")
		}
	})

	// The headline of ADR-0008: the MiG-21 still flies when the ground station
	// is gone. Othela is stopped, a commit is pushed, and the agent runs it.
	It("keeps pulling and running with Othela stopped", func() {
		_, err := compose(time.Minute, "stop", "othela")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_, err := compose(3*time.Minute, "up", "--detach", "--wait", "othela")
			Expect(err).NotTo(HaveOccurred())
		})

		sha, err := seedCommit("nginx-collection", "NOTES.md", "pushed while Othela was stopped")
		Expect(err).NotTo(HaveOccurred())

		for _, node := range agentNodes {
			Eventually(func() (agentRunStatus, error) { return lastRun(node) }, 4*time.Minute, 5*time.Second).
				Should(And(HaveField("CommitSHA", sha), HaveField("Status", "Success")),
					node+" did not run the commit pushed while Othela was down")
		}
	})
})

var _ = Describe("Othela, frozen", func() {
	// ADR-0006 in the running stack: the loader must account for every path it
	// declined, not only report a count. Issue #34 was filed because a count of
	// zero arrived with nothing to act on.
	It("accounts for the paths the loader skipped", func() {
		logs, err := journal("othela", "helvilette-othela")
		Expect(err).NotTo(HaveOccurred())

		Expect(logs).To(ContainSubstring(`"SKIP_REASON":"hidden_dir"`),
			"the fleet cache contains .git; the loader did not say it skipped it")
		Expect(logs).To(ContainSubstring(`"FILES_EXAMINED"`),
			"the scan summary did not say how many files it looked at")
	})
})

var _ = Describe("Units and journald", func() {
	// Helvilette manages systemd, so the e2e stack runs it under systemd. A
	// foreground-process container cannot exercise either this or the journal.
	// See ADR-0007 R2.
	It("runs Othela and the agents as active systemd units", func() {
		units := map[string]string{"othela": "helvilette-othela.service"}
		for _, node := range agentNodes {
			units[node] = "helvilette-agent.service"
		}
		for service, unit := range units {
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
			Expect(out).To(ContainSubstring(`"COMPONENT"`), "journal entries carry no Helvilette fields")
		}
	})

	// The headline of ADR-0007 D2. Before it, every key sat inside one opaque
	// MESSAGE, so a field query matched nothing and a collector needed a JSON
	// parse stage before any of this was usable.
	It("makes Helvilette's own fields queryable", func() {
		queries := []struct {
			service, field, value string
		}{
			{"node-1", "NODE_ID", "node-1"},
			{"othela", "COMPONENT", "othela"},
			{"othela", "COMPONENT", "playbook-loader"},
		}

		for _, q := range queries {
			out, err := journalMatching(q.service, q.field, q.value)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(out)).NotTo(BeEmpty(),
				"journalctl "+q.field+"="+q.value+" returned nothing on "+q.service)
		}
	})

	// MESSAGE must read as prose, not as a JSON document. That is what makes
	// `journalctl` and `make journal` legible to a human, and it is the half of
	// D2 that a field query cannot prove.
	It("writes messages as prose rather than embedded JSON", func() {
		out, err := journalMatching("othela", "COMPONENT", "othela")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("starting othela"))
		Expect(out).NotTo(ContainSubstring(`"level":`), "the whole JSON line is still inside MESSAGE")
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
