package e2e_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startupTimeout bounds every container readiness wait. Set explicitly rather
// than relying on the testcontainers default, so a slow host fails with a clear
// deadline rather than an ambiguous one. See ADR-0003.
const startupTimeout = 3 * time.Minute

// repoRootPath resolves the repository root, used as the Docker build context
// for every image the suite builds.
func repoRootPath() string {
	root, err := filepath.Abs("../../")
	Expect(err).NotTo(HaveOccurred())
	return root
}

var _ = Describe("GitOps Workflow", func() {
	var ctx context.Context
	var gitContainer testcontainers.Container
	var othelaContainer testcontainers.Container
	var agentContainer testcontainers.Container
	var agent2Container testcontainers.Container
	var network testcontainers.Network

	BeforeEach(func() {
		ctx = context.Background()

		// 0. Create a network for containers to communicate
		networkName := fmt.Sprintf("helvilette-e2e-net-%d", time.Now().UnixNano())
		var err error
		network, err = testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
			NetworkRequest: testcontainers.NetworkRequest{
				Name: networkName,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		// 1. Setup Git Server (git daemon)
		absPlaybookPath, err := filepath.Abs("./data/playbooks")
		Expect(err).NotTo(HaveOccurred())

		gitReq := testcontainers.ContainerRequest{
			// Built from Dockerfile.gitserver rather than installing git at
			// container start. The previous `apk add` made every run depend on a
			// package download finishing inside the startup deadline, which failed
			// under load and passed on an idle host. See ADR-0003.
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRootPath(),
				Dockerfile: "Dockerfile.gitserver",
			},
			ExposedPorts: []string{"9418/tcp"},
			Mounts: testcontainers.ContainerMounts{
				{
					Source:   testcontainers.GenericBindMountSource{HostPath: filepath.Join(absPlaybookPath, "nginx-collection")},
					Target:   testcontainers.ContainerMountTarget("/git/nginx-collection"),
					ReadOnly: true,
				},
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"git-server"},
			},
			WaitingFor: wait.ForLog("Ready to rumble").WithStartupTimeout(startupTimeout),
		}
		
		gitContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: gitReq,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())

		// 2. Setup Othela Control Plane
		repoRoot := repoRootPath()

		othelaReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot,
				Dockerfile: "Dockerfile.othela",
			},
			ExposedPorts: []string{"8080/tcp"},
			Cmd: []string{
				"./othela",
				"--port=8080",
				"--fleet-repo=git://git-server:9418/nginx-collection",
				// State stays on the container filesystem. Bind-mounting it would
				// put a writable path inside the Go module tree, which is what
				// broke `go vet ./...` before ADR-0003.
				"--state-dir=/app/state",
				"--log-level=debug",
			},
			Env: map[string]string{
				"HELV_TEST_REPO_URL": "git://git-server:9418/nginx-collection",
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"othela"},
			},
			// Wait for HTTP endpoint to be ready
			WaitingFor: wait.ForHTTP("/api/v1/playbooks").WithPort("8080/tcp").WithStartupTimeout(startupTimeout),
		}

		othelaContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: othelaReq,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())

		// 3. Setup Agent
		agentReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot,
				Dockerfile: "Dockerfile.agent",
			},
			Env: map[string]string{
				"OTHELA_URL": "http://othela:8080",
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"agent-01"},
			},
			// Create a config file via command or ENV for the agent
			Cmd: []string{
				"./agent",
				"--othela-url=http://othela:8080",
				"--node-id=agent-01",
				"--poll-interval=5s",
				"--workspace-dir=/tmp/helvilette",
				"--labels=role=edge-proxy",
			},
		}
		agentContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: agentReq,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())

		// 4. Setup Agent 2 (Unmatched labels)
		agentReq2 := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot,
				Dockerfile: "Dockerfile.agent",
			},
			Env: map[string]string{
				"OTHELA_URL": "http://othela:8080",
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"agent-02"},
			},
			Cmd: []string{
				"./agent",
				"--othela-url=http://othela:8080",
				"--node-id=agent-02",
				"--poll-interval=5s",
				"--workspace-dir=/tmp/helvilette",
				"--labels=role=database",
			},
		}
		agent2Container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: agentReq2,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Cleanup containers
		if agent2Container != nil {
			agent2Container.Terminate(ctx)
		}
		if agentContainer != nil {
			agentContainer.Terminate(ctx)
		}
		if othelaContainer != nil {
			othelaContainer.Terminate(ctx)
		}
		if gitContainer != nil {
			gitContainer.Terminate(ctx)
		}
		if network != nil {
			network.Remove(ctx)
		}
	})

	It("Should pull playbook from git and execute on agent", func() {
		// We expect the agent to sync, execute ansible, and report back to othela.
		// Since we just started it, the agent should eventually process the job.
		// Let's poll othela's API or agent's logs.
		// For now, let's poll othela API directly via its mapped port to see if a report was filed.
		
		hostIP, err := othelaContainer.Host(ctx)
		Expect(err).NotTo(HaveOccurred())

		mappedPort, err := othelaContainer.MappedPort(ctx, "8080/tcp")
		Expect(err).NotTo(HaveOccurred())

		othelaURL := fmt.Sprintf("http://%s:%s/api/v1/playbooks", hostIP, mappedPort.Port())

		Eventually(func() error {
			resp, err := http.Get(othelaURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != 200 {
				return fmt.Errorf("Status not 200: %d", resp.StatusCode)
			}
			
			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)
			// Wait until playbooks are available
			if len(bodyStr) < 5 {
				return fmt.Errorf("Playbooks empty")
			}
			
			return nil
		}, 30*time.Second, 2*time.Second).Should(Succeed())

		// Optionally check agent logs to confirm sync
		Eventually(func() string {
			logs, err := agentContainer.Logs(ctx)
			if err != nil {
				return ""
			}
			logBytes, _ := io.ReadAll(logs)
			return string(logBytes)
		}, 3*time.Minute, 5*time.Second).Should(ContainSubstring("playbook execution"))

		// Check othela logs to confirm it gracefully ignores agent 2
		Eventually(func() string {
			logs, err := othelaContainer.Logs(ctx)
			if err != nil {
				return ""
			}
			logBytes, _ := io.ReadAll(logs)
			return string(logBytes)
		}, 30*time.Second, 2*time.Second).Should(ContainSubstring("[DEBUG] Node agent-02 has labels"))
	})

	It("Should expose health and readiness endpoints on Othela", func() {
		hostIP, err := othelaContainer.Host(ctx)
		Expect(err).NotTo(HaveOccurred())

		mappedPort, err := othelaContainer.MappedPort(ctx, "8080/tcp")
		Expect(err).NotTo(HaveOccurred())

		baseURL := fmt.Sprintf("http://%s:%s", hostIP, mappedPort.Port())

		// Test /healthz
		resp, err := http.Get(baseURL + "/healthz")
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		bodyBytes, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(bodyBytes)).To(ContainSubstring(`"status":"ok"`))

		// Test /readyz
		resp2, err := http.Get(baseURL + "/readyz")
		Expect(err).NotTo(HaveOccurred())
		defer resp2.Body.Close()
		Expect(resp2.StatusCode).To(Equal(http.StatusOK))

		bodyBytes2, err := io.ReadAll(resp2.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(bodyBytes2)).To(ContainSubstring(`"status":"ok"`))
	})
})
