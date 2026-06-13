# Helvilette: An OS Service Orchestration Framework.

Helvilette is a service orchestration framework that uses Ansible Playbooks to manage services on a fleet of nodes. It is designed to be a lightweight and flexible alternative to traditional configuration management tools.

Helvilette is a project aiming to join the [Cloud Native Computing Foundation (CNCF)](https://cncf.io).

## Getting Started

<!-- Include enough details to get started using, or at least building, the
project here and link to other docs with more detail as needed.  Depending on
the nature of the project and its current development status, this might
include:
* quick installation/build instructions
* a few simple examples of use
* basic prerequisites
--> 

## Development Setup

### Method A: Manual installation on Linux Environment (WSL/Linux)

We will open two terminals in WSL to simulate the Server and the Agent.

#### Terminal 1: Othela (Control Plane)

Start the server listening on port 8080:

```bash
cd /mnt/e/Helvilette
/usr/local/go/bin/go run ./cmd/othela --port=8080 --data-dir=helvillette/othela/data/playbooks --log-level=info
```

#### Terminal 2: Helvilette Agent

Start the agent. It will poll Othela every 5 seconds.
You can use environment variables, CLI flags, or a config file (Kubelet style).

```bash
cd /mnt/e/Helvilette
# Using CLI flags
/usr/local/go/bin/go run ./cmd/agent --othela-url=http://localhost:8080/api/v1 --node-id=agent-local --poll-interval=5s
```

### Expected Behavior
1. **Agent** connects to Othela.
2. **Othela** sends a "Job" containing an Ansible Playbook (Prints "Hello Wunjo!").
3. **Agent** saves this to its configured workspace directory.
4. **Agent** runs `ansible-playbook -i "localhost," -c local` with `ANSIBLE_STDOUT_CALLBACK=json`.
5. **Agent** captures the JSON output and sends it back to Othela.
6. **Othela** prints the JSON report to the console.

### Method B: Automated E2E Testing with Ginkgo and Testcontainers (Recommended)

Helvilette uses a BDD testing framework ([Ginkgo](https://onsi.github.io/ginkgo/)) combined with [Testcontainers-Go](https://golang.testcontainers.org/) to run end-to-end (E2E) tests.  

When you run the E2E suite, it will:
1. Automatically build a lightweight `git-daemon` container serving test playbooks over `git://`.
2. Build the `othela` (Control Plane) and `agent` container images directly from the local Dockerfiles.
3. Assert the state and outputs of the GitOps reconciliation loop programmatically.
4. Clean up all containers and networks automatically.

**Prerequisites:**
You need `ginkgo` installed on your machine:
```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

**Running the tests:**
```bash
# Run the complete E2E test suite
make e2e
```

## Contributing
<!-- Template: https://github.com/cncf/project-template/blob/main/CONTRIBUTING.md -->

Our project welcomes contributions from any member of our community. To get
started contributing, please see our [Contributor Guide](TODO: Link to
CONTRIBUTING.md).

## Scope
<!-- If this section is too long, you might consider moving it to a SCOPE.md -->
<!-- More information about creating your scope with links to examples -->
<!-- https://contribute.cncf.io/maintainers/governance/charter/ -->

### In Scope

### In Scope

Helvilette is intended to provide a pull-based delivery layer for Ansible. As such, the project will implement or has implemented:

* Desired-state reconciliation loop at the OS/systemd level
* Pull-based GitOps model without inbound SSH access
* Lightweight agent architecture suitable for edge computing and IoT devices

### Out of Scope

Helvilette will be used in a cloud native environment with other tools. The following specific functionality will therefore not be incorporated:

* Container orchestration (like Kubernetes or Docker Swarm)
* General-purpose CI/CD pipelines (like GitHub Actions or GitLab CI)
* Core configuration management capabilities (Ansible performs this function)

Helvilette implements an OS service orchestration framework, through an API-driven control plane (Othela) and local node agents written in Go. It will not cover infrastructure provisioning (e.g., Terraform or Pulumi).

## Communications

* User Mailing List:
* Developer Mailing List:
* Slack Channel:
* Public Meeting Schedule and Links: 
* Social Media:
* Other Channel(s), If Any:

## Resources

[TODO: Add links to other helpful information (roadmap, docs, website, etc.)]

## License

This project is licensed under [Apache License Version 2.0](https://www.apache.org/licenses/LICENSE-2.0).

## Code of Conduct

We follow the CNCF Community Code of Conduct v1.3 stated at [https://github.com/cncf/foundation/blob/main/code-of-conduct.md](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).