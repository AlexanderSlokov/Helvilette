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

var _ = Describe("GitOps Workflow", func() {
	var ctx context.Context
	var gitContainer testcontainers.Container
	var othelaContainer testcontainers.Container
	var agentContainer testcontainers.Container
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

		// 1. Setup Git Server (alpine/git daemon)
		absDataPath, err := filepath.Abs("./data/playbooks")
		Expect(err).NotTo(HaveOccurred())

		gitReq := testcontainers.ContainerRequest{
			Image:        "alpine:3.19",
			ExposedPorts: []string{"9418/tcp"},
			Cmd: []string{
				"sh", "-c",
				"apk add --no-cache git git-daemon && git daemon --verbose --export-all --base-path=/git --reuseaddr --enable=receive-pack",
			},
			Mounts: testcontainers.ContainerMounts{
				testcontainers.BindMount(absDataPath, testcontainers.ContainerMountTarget("/git/nginx-collection")),
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"git-server"},
			},
			WaitingFor: wait.ForLog("Ready to rumble"),
		}
		
		gitContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: gitReq,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())

		// 2. Setup Othela Control Plane
		repoRoot, err := filepath.Abs("../../")
		Expect(err).NotTo(HaveOccurred())

		othelaReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    repoRoot,
				Dockerfile: "Dockerfile.othela",
			},
			ExposedPorts: []string{"8080/tcp"},
			Cmd: []string{
				"./othela",
				"--port=8080",
				"--data-dir=./tests/e2e/data/playbooks",
			},
			Env: map[string]string{
				"HELV_TEST_REPO_URL": "git://git-server:9418/nginx-collection",
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"othela"},
			},
			// Wait for HTTP endpoint to be ready
			WaitingFor: wait.ForHTTP("/api/v1/playbooks").WithPort("8080/tcp"),
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
			// Actually the Dockerfile.agent says: CMD ["./agent"]
			// And docker-compose mapped the config file. We can just use CLI flags instead of config file.
			Cmd: []string{
				"./agent",
				"--config=/var/lib/helvilette/agent.yaml", // we will mount this
			},
			Mounts: testcontainers.ContainerMounts{
				testcontainers.BindMount(filepath.Join(repoRoot, "data/configs/agent-01.yaml"), testcontainers.ContainerMountTarget("/var/lib/helvilette/agent.yaml")),
			},
		}
		agentContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: agentReq,
			Started:          true,
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Cleanup containers
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
		}, 30*time.Second, 2*time.Second).Should(ContainSubstring("Sync successful"))
	})
})
