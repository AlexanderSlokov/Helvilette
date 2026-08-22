# Helvilette - Declarative Continuous Delivery for Ansible Playbooks.

Helvilette is a declarative GitOps continuous delivery tool for Ansible that uses **your** Ansible Playbooks to manage desired state on a fleet of nodes. It is designed to be a lightweight and flexible alternative to deploying Ansible configurations via GitOps pull-based model at scale, or at edge.

## Why Helvilette?

The current cloud-native era has created an invisible divide:

- **Tier 1 (~20%):** Organizations with Kubernetes, ArgoCD, Flux, and a full GitOps ecosystem. Config drifts are handled by K8s reconciliation loops.
- **Tier 2 (~80%):** Startups, SMBs, universities, government agencies, and homelabs running 5–50 VMs on bare metal, Proxmox, or scattered VPS providers. Config drifts happen daily, and without Kubernetes, there is no easy way to fix them.

For Tier 2 users, the current Ansible workflow — "runnable by CI/CD pipeline" — is a maze of pain:

```text
Playbook ready
    │
    ├─► Push to Git
    ├─► Configure GitHub Actions / GitLab CI
    ├─► Setup SSH keys in CI secrets
    ├─► Open port 22 (or hack a bastion host / VPN tunnel)
    ├─► Write CI pipeline YAML calling ansible-playbook
    ├─► Debug why the CI runner can't SSH into some nodes
    ├─► 4 hours scratching your head + 4 more for a post-mortem
    │
    ▼
Server maybe configured???
```

Each step is "glue work". Nobody enjoys it, nobody excels at it, and critically, nobody should be exposing SSH port 22 just to run a playbook. The push-based model forces a fundamental security anti-pattern: all SSH root keys concentrated on a single laptop or CI server. Laptop stolen? Entire infrastructure compromised. Engineer quits? Infrastructure knowledge walks out the door.

**Helvilette eliminates the push-based delivery pipeline entirely:**

```text
Playbook ready
    │
    ├─► Push to Git repo
    ▼
Othela notifies and sets Job specs
    │
    ▼
Agent polls → Agent clones → Agent runs ansible-playbook → Agent reports back
```

> *"Use SSH for Ansible to install Helvilette once. Then never need SSH for Ansible again."*

### Architecture:

Helvilette maps 1:1 to Kubernetes architecture, transposed to the OS/systemd layer:

| Kubernetes Concept | Helvilette Equivalent | Role |
|---|---|---|
| `kube-apiserver` | **Othela** (Control Plane) | Receives declarations, dispatches jobs to agents |
| `kubelet` | **Helvilette Agent** | Sits on each node, pulls + executes + reports |
| OCI Image | **Ansible Playbook Repo** | Artifact containing execution logic |
| Container Registry | **Git Server** | Artifact storage |
| `Dockerfile` | **`playbook.yml` + `roles/`** | Defines "what needs to be done" |
| `values.yaml` (Helm) | **`helvilette.yml`** | Per-deployment declarative configuration |
| Pod spec / `nodeSelector` | **`nodeGroup` / `nodeSelector`** | Declares what runs where |
| `livenessProbe` | **`livenessProbe`** | Identical concept, applied to systemd services |
| Container runtime | **Ansible engine** | The actual executor |
| `etcd` | **SQLite / Git** | State storage |

### The Bootstrap Elegance

Ansible installs Helvilette (the last SSH session). Helvilette delivers all future Ansible playbooks. The chicken lays the egg-making machine, then retires.

```text
Last SSH session ever:
┌──────────────────────────────────────────┐
│  ansible-playbook install-helvilette.yml │
│                                          │
│  → Installs agent on N servers           │
│  → Agent registers with Othela           │
│  → systemd enable + start                │
│                                          │
│  Done. Close port 22. Forever.           │
└──────────────────────────────────────────┘
```

### The Immune System Layer

Helvilette operates at the OS/systemd layer — beneath Kubernetes, beneath container runtimes, beneath everything:

```text
Layer 4:  ┌─ Kubernetes ───────────────────────────┐
          │  Pods, Deployments, Services           │
          │  ❌ Cannot self-heal                   │
Layer 3:  ├─ Container Runtime (containerd) ───────┤
          │  ❌ Cannot self-restart                │
Layer 2:  ├─ systemd ──────────────────────────────┤
          │  kubelet.service, containerd.service   │
          │  etcd.service, kube-apiserver.service  │
Layer 1:  ├─ OS (Linux) ───────────────────────────┤
          │                                        │
          │  🐈‍⬛ Helvilette Agent lives here.       │
          │  It is a systemd service.              │
          │  It can see EVERYTHING above.          │
          └────────────────────────────────────────┘
```

This gives Helvilette a unique capability: **managing the managers**. It can rolling-update `kubelet`, restart `kube-apiserver`, and heal what K8s cannot heal — because K8s cannot perform surgery on its own brain.

### Key Features

- **Pull-based GitOps delivery** — Agents pull playbooks from Git. No inbound SSH access required.
- **Desired-state reconciliation loop** — Continuous drift detection and self-healing at the OS/systemd level.
- **Lightweight agent architecture** — Single Go binary (~20MB RAM), runs on Raspberry Pi (ARM64). Suitable for edge computing and IoT devices.
- **K8s-familiar configuration** — `helvilette.yml` uses `apiVersion`, `kind`, `metadata`, `spec`, `nodeSelector`, and probe conventions you already know.
- **Label-based node targeting** — Route jobs to the right nodes using `nodeSelector` labels, just like Kubernetes pod scheduling.
- **Zero vendor lock-in** — Remove Helvilette and you still have working Ansible playbooks + Git repos. No proprietary DSL.
- **AI-ready** — `helvilette.yml` uses K8s + Ansible conventions massively represented in LLM training data; any AI can generate valid configs immediately.
- **Structured reporting** — Agents capture Ansible JSON output and report execution results back to the control plane.

### How Does Helvilette Compare to Existing Tools?

| Tool | Relationship to Helvilette |
|---|---|
| **Puppet** | Same pull-based architecture. But Puppet requires a proprietary DSL, Hiera, Facter, and PuppetDB. Helvilette uses YAML + Ansible — zero new learning. |
| **Ansible AWX / Semaphore UI** | Still push-based. SSH keys moved from laptop to AWX server — attack surface relocated, not eliminated. No continuous reconciliation. |
| **GitHub / GitLab Runner** | CI runner asks "where should this code run?". Helvilette asks "what should this server look like?". Imperative vs. convergent. |
| **Kubernetes** | K8s manages containers. Helvilette manages systemd services. K8s cannot self-heal its own control plane. Helvilette can do that for k8s, on a daily basic. |
| **SaltStack** | Replaces Ansible entirely with its own language. Helvilette preserves your existing Ansible investment. |

## Getting Started

### Prerequisites

- **Go** 1.25+ ([download](https://go.dev/dl/))
- **Ansible** installed on every machine that will run the Helvilette Agent
- **Git** installed on agent machines (for playbook cloning)
- **Docker** (optional, for running the E2E test suite or the ephemeral lab)

### Quick Start: Build from Source

```bash
# Clone the repository
git clone https://github.com/AlexanderSlokov/Helvilette.git
cd Helvilette

# Build both binaries
make build

# Binaries are output to ./bin/
ls bin/
# othela  agent
```

### Declarative Configuration: `helvilette.yml`

Helvilette uses a declarative manifest (placed inside your Ansible playbook repository) that "looks like" Kubernetes API conventions and keeps the semantic meaning & behavior as close as possible to K8s's declarative model.

For example:

```yaml
apiVersion: apps/v1
kind: Cluster
metadata:
  name: "my-company-edge-proxy-fleet"
  labels:
    app: "nginx-collection"
    version: "1.0.0"

spec:
  repo: "http://git-server:3000/helvilette/nginx-collection.git"
  branch: "main"
  playbook: "playbook.yml"

  nodeGroups:
    - name: "standard-proxies"
      nodeSelector:
        role: "edge-proxy"
      ansible:
        extra_vars:
          nginx_http_port: "80"
          nginx_https_port: "443"
```

Agents with matching labels (e.g., `role=edge-proxy`) will automatically pull and execute the configured playbook. Agents with non-matching labels will receive no work, and chillax.

### Agent Configuration

The Agent supports K8s-style configuration with the following priority: **CLI flags > YAML config > Environment variables > Defaults**.

Note that the config file outranks environment variables, matching k3s. The file is an explicit,
version-controlled artifact, so what you read there is what the agent runs — an ambient `OTHELA_URL`
inherited from a systemd unit or container environment will not silently override it. See
[ADR-0001](docs/informations/ADRs/ADR-0001.md) for the reasoning.

**Using CLI flags:**

```bash
./bin/agent \
  --othela-url=http://othela-server:8080/api/v1 \
  --node-id=node-01 \
  --poll-interval=5s \
  --labels="role=edge-proxy,env=production"
```

**Using environment variables:**

```bash
export OTHELA_URL=http://othela-server:8080/api/v1
export NODE_ID=node-01
export POLL_INTERVAL=5s
export AGENT_LABELS=role=edge-proxy,env=production
./bin/agent
```

**Using a YAML config file** ("taste" like kubelet):

```yaml
# /var/lib/helvilette/agent.yaml
othelaURL: "http://othela-server:8080/api/v1"
nodeID: "node-01"
pollInterval: "5s"
workspaceDir: "/var/lib/helvilette/workspace"
labels:
  role: "edge-proxy"
  env: "production"
```

Keys are matched exactly as written above. Unrecognised keys are rejected at startup rather
than ignored, so a typo surfaces as an error instead of an agent silently running on defaults.

And then, start the agent with config file:

```bash
./bin/agent --config=/var/lib/helvilette/agent.yaml
```

### Expected Workflow

1. **Agent** starts and registers with Othela (sends its `nodeId` + `labels`).
2. **Agent** enters a polling loop, asking Othela for work at the configured interval.
3. **Othela** matches the agent's labels against `nodeSelector` rules in `helvilette.yml` and returns a Job with a Git repo reference + `extra_vars`.
4. **Agent** clones/pulls the playbook repository to its local workspace.
5. **Agent** runs `ansible-playbook` with `ANSIBLE_STDOUT_CALLBACK=json` and any configured `extra_vars`.
6. **Agent** captures the structured JSON output and reports the result back to Othela.

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

Helvilette is intended to provide a pull-based delivery platform for Ansible. As such, the project will implement or has implemented:

* Desired-state reconciliation loop at the OS/systemd level
* Pull-based GitOps model without inbound SSH access
* Lightweight agent architecture suitable for edge computing and IoT devices

### Out of Scope

Helvilette will be used in a cloud native environment or non cloud-native environments with other tools. Regardless, the following specific functionality will therefore not be incorporated:

* Container orchestration (like Kubernetes or Docker Swarm)
* General-purpose CI/CD pipelines (like GitHub Actions or GitLab CI)
* Core configuration management capabilities (Ansible performs this function)
* Infrastructure provisioning (e.g., should use Terraform or Pulumi)

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