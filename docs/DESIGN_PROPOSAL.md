# Design Proposal: Helvilette — Pull-Based Ansible Delivery & Drift Protection Layer

**Authors**: Aleksandr S. Naughtian <dinhtandung.work@gmail.com>

**Begin Design Discussion**: 2025-08-16

**(optional) Status**: implementable

**Checklist**:

- [x] Minimum Viable Product
- [x] Design document
- [x] Control Plane is ready
- [x] Agent is ready
- [ ] Docs
- [x] Tests

## Summary/Abstract

Helvilette is a pull-based delivery layer for Ansible — it turns Ansible playbooks into a self-operating system that runs without SSH, without CI/CD pipeline glue, and without anyone pressing Enter.

Helvilette is not an orchestrator, and will not try to compete with `Kubernetes`, `Nomad`, `Puppet`, or `SaltStack`. It operates at the OS/systemd layer - beneath all orchestrators - providing desired-state reconciliation for the 80% of infrastructure that will never run Kubernetes.

> *"Use Ansible to install Helvilette once. Never need SSH for Ansible again."*

## Background

### Motivation and problem space

The current cloud-native era has created an invisible divide:

- **Tier 1 (20%):** Organizations with `Kubernetes`, `ArgoCD`, `Flux`, `Istio`, `GitOps` tools. Config drifts are handled by K8s reconciliation loops.
- **Tier 2 (80%):** Startups, SMBs, universities, government agencies, homelabs running 5-50 VMs on bare metal, `Proxmox`, or scattered VPS providers. Config drifts happen as daily basics. Without `Kubernetes`, there is no easy way to fix them.

For user in Tier 2, the current Ansible workflow, which is "runable by CI/CD pipeline", is a maze of pain:

```text
Playbook ready
    │
    ├─► Push to Git
    ├─► Configure GitHub Actions / GitLab CI
    ├─► Setup SSH keys in CI secrets
    ├─► Open port 22 (or hack a bastion host / VPN tunnel)
    ├─► Write CI pipeline YAML calling ansible-playbook
    ├─► Debug why the CI runner can't SSH
    ├─► 4 hours scratching your head and more 4 hours for a post-mortem.
    │
    ▼
Server maybe configured???
```

Each step is a "glue work". Nobody enjoys it, nobody excels at it, and — critically — nobody should be exposing SSH port 22 just to run a playbook.

The push-based model forces a fundamental security anti-pattern: all SSH root keys concentrated on a single laptop or CI server. Laptop stolen? Entire infrastructure compromised. Engineer quits? Infrastructure knowledge walks out the door.

### Impact and desired outcome

Helvilette eliminates the push-based delivery pipeline entirely:

```text
Playbook ready
    │
    ├─► Push to Git repo
    │
    ▼
Othela notify and set Job specs
    │
    ▼
Agent polls
    │
    ▼
Agent clones
    │
    ▼
Agent runs ansible-playbook
    │
    ▼
Agent reports back to Othela API
```

**Desired outcomes:**
1. **Zero SSH keys** on any laptop or CI server
2. **Continuous reconciliation** — agents poll and self-correct drift automatically
3. **Zero learning curve** — engineers who know Ansible + K8s YAML can operate Helvilette immediately
4. **Bus factor elimination** — infrastructure knowledge lives in Git repos + running agents, not in anyone's head
5. **Infrastructure immune system** — Helvilette can heal services that orchestrators cannot heal themselves (including K8s control plane components)

### Prior discussion and links

- Development Journal: [JOURNAL.md](./JOURNAL.md)
- Roadmap & Architecture Decisions: [TODO.md](./TODO.md)

## User Story

### Primary Persona: "Tuấn" — Solo DevOps at a Vietnamese Startup

**Context:** Series A startup, 15-30 VMs across AWS, Proxmox, and budget VPS providers (Mắt Bão, BKNS, Viettel IDC). Team: 1 infrastructure engineer.

**Current pain:**
- `~/.ssh` contains 14+ root private keys on a personal laptop
- Ansible playbooks exist in Git but only Tuấn knows how to run them
- CI/CD pipeline for Ansible deployment is brittle and requires SSH exposure
- Config drift goes undetected until something breaks
- If Tuấn quits, infrastructure knowledge leaves with him

**With Helvilette:**
- Day 0: Tuấn installs Helvilette agents on all servers (last SSH session)
- Day 1: Agents self-configure by pulling playbooks from Git
- Day 15: SSL renewal playbook auto-runs. No human intervention.
- Day 30: New VMs? Install agent, point to Othela. Auto-configured.
- Day 45: Junior modifies nginx.conf by hand → agent detects drift, re-applies playbook
- Day 60: Tuấn quits. Infrastructure self-operates. New hire only needs Git push access.

### Secondary Persona: Homelab / Edge Infrastructure Operator

**Context:** 5-20 devices (Raspberry Pi, mini PCs, VPS), hybrid infra, running k3s, Nginx, monitoring stacks.

**With Helvilette:**
- Single Go binary per node (ARM64 compatible)
- `helvilette.yml` declares desired state like a `Helm chart`
- Agent monitors systemd services with K8s-style probes
- If k3s crashes at 3 AM, agent restarts it. If restart fails, `Grafana Alloy` (recommended to integrate with Helvilette) alerts `Grafana Cloud`.

## Goals

1. **Eliminate SSH from Ansible workflows** — pull-based delivery removes the need for any inbound SSH access
2. **Provide continuous desired-state reconciliation** for systemd services on bare-metal/VM infrastructure
3. **Zero learning curve** — reuse Ansible as execution engine, K8s API conventions for configuration
4. **Lightweight deployment** — single Go binary for both Othela and Agent; runs on Raspberry Pi
5. **GitOps-native from day one** — playbooks stored in Git, agents pull on change
6. **Infrastructure immune system** — operate at OS/systemd layer to heal services that higher-level orchestrators cannot self-heal

## Non-Goals

1. **Not an orchestrator** — Helvilette does not replace Kubernetes, Nomad, or Docker Swarm
2. **Not a CI/CD system** — Helvilette does not replace GitHub Actions or GitLab CI
3. **Not an Ansible replacement** — Ansible remains the execution engine; Helvilette only handles delivery and reconciliation
4. **Not a configuration management language** — Helvilette uses Ansible playbooks and K8s-style YAML; no proprietary DSL
5. **Not a container orchestrator** — Helvilette manages OS-level systemd services, not containers

## Proposal

### Architecture Overview

Helvilette maps 1:1 to Kubernetes architecture, transposed to the OS/systemd layer:

| Kubernetes | Helvilette | Role |
|---|---|---|
| kube-apiserver | **Othela** | Control plane, receives declarations, dispatches to agents |
| kubelet | **Helvilette Agent** | Sits on node, pulls + executes + reports |
| OCI Image | **Ansible Playbook Repo** | Artifact containing execution logic |
| Container Registry | **Git Server** | Artifact storage |
| Dockerfile | **playbook.yml + roles/** | Defines "what needs to be done" |
| values.yaml (Helm) | **helvilette.yml** | Per-deployment configuration |
| Pod spec | **nodeGroup** | Declares what runs where |
| nodeSelector | **nodeSelector** | Identical concept |
| livenessProbe | **livenessProbe** | Identical concept, applied to systemd services |
| Container runtime | **Ansible engine** | The actual executor |
| etcd | **SQLite / Git** | State storage |

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

```
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
          │  🐈‍Helvilette Agent lives here.        │ 
          │  It is a systemd service.              │
          │  It can see EVERYTHING above.          │
          └────────────────────────────────────────┘
```

This gives Helvilette a unique capability: **managing the managers**. It can rolling-update `kubelet`, restart `kube-apiserver`, and heal what K8s cannot heal — because K8s cannot perform surgery on its own brain.

## Design Details

### `helvilette.yml` — The Helm Chart for Bare Metal

The declarative configuration uses K8s API conventions, near-zero learning curve:

```yaml
apiVersion: apps/v1
kind: Cluster
metadata:
  name: "my-company-edge-proxy-fleet"
  namespace: "default"
  labels:
    app: "nginx-collection"
    version: "1.0.0"

spec:
  repo: "http://git-server:3000/helvilette/nginx-collection.git"
  branch: "main"
  playbook: "playbook.yml"

  vault:
    nginx_vault_secret:
      type: exported
      source: "H8E_VAULT_SECRET"
      target: "group_vars/all/vault.yaml"

  nodeGroups:
    - name: "standard-proxies"
      nodeSelector:
        role: "edge-proxy"
      ansible:
        vault-password-file: nginx_vault_secret
        extra_vars:
          nginx_http_port: "80"
          nginx_https_port: "443"
      probes:
        nginx.service:
          livenessProbe:
            httpGet:
              path: /healthz
              port: 80
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
        mariadb.service:
          readinessProbe:
            exec:
              command: ["mysqladmin", "ping", "-h", "localhost"]
            periodSeconds: 15
            failureThreshold: 2
```

**Key design decisions:**
- `apiVersion`, `kind`, `metadata`, `spec` — Kubernetes API conventions (Apache 2.0, freely adoptable)
- `nodeSelector` — identical to K8s pod scheduling
- `livenessProbe` / `readinessProbe` — K8s health-checking concepts applied to systemd services (a feature Puppet never had)
- `vault` — pluggable secret management (env, HashiCorp Vault, K8s secrets, Docker secrets)
- Playbook repo referenced by Git URL — agent pulls like kubelet pulls OCI images

### Othela Control Plane

```
REST API:
  GET  /api/v1/sync/{node_id}     → Returns Job spec for this node
  POST /api/v1/report             → Receives execution reports
  GET  /api/v1/playbooks          → Lists available playbooks
  POST /api/v1/repos              → Register new playbook repository
  GET  /api/v1/repos              → List registered repositories
```

- Go binary with Cobra CLI flags (`--port`, `--data-dir`, `--log-level`)
- Playbook loader discovers collections from disk
- SQLite persistence (planned) for job history, node registry, reports
- Webhook endpoint for Git push notifications

### Helvilette Agent

- Go binary with K8s-style configuration: CLI flags > YAML config > ENV > Defaults
- Polling loop (configurable interval, default 5s)
- Git clone/pull via `go-git` library — local repo caching
- Ansible execution via `exec.Command` with `ANSIBLE_STDOUT_CALLBACK=json`
- Systemd D-Bus watcher for real-time service state monitoring
- Structured reporting with zerolog

### K8s Patterns Available for Adoption (Apache 2.0)

| Pattern | K8s Source | Helvilette Application |
|---|---|---|
| `CrashLoopBackOff` | Pod restart backoff | Systemd service keeps failing |
| `GitPullBackOff` | `ImagePullBackOff` | Repo clone/pull keeps failing |
| `PlaybookFailBackOff` | — | ansible-playbook keeps failing |
| `DaemonSet` | Run on every node | Run playbook on all nodes |
| `Job` | One-time execution | Run-once tasks (DB migration, backup) |
| `CronJob` | Scheduled execution | Scheduled tasks (SSL renewal, cleanup) |
| Taints & Tolerations | Scheduling constraints | Production protection |
| Events | Audit trail | PlaybookApplied, DriftDetected, SelfHealed |
| Rolling Update Strategy | Deployment strategy | Update nodes sequentially with health checks |
| Annotations | Metadata | `helvilette.io/last-applied`, `helvilette.io/rollback-to` |

## Impacts / Key Questions

### Positioning: Why Not Use Existing Tools?

| Tool | Relationship to Helvilette |
|---|---|
| **Puppet** | Same architecture (pull-based, agent, reconciliation). Different everything else: Puppet requires proprietary DSL, Hiera, Facter, PuppetDB. Helvilette uses YAML + Ansible — zero new learning. |
| **Ansible AWX / Semaphore UI** | Still push-based. SSH keys moved from laptop to AWX server — attack surface relocated, not eliminated. No continuous reconciliation. No health probes. |
| **GitHub / GitLab Runner** | CI runner asks "where should this code run?". Helvilette asks "what should this server look like?". Imperative vs convergent. |
| **Kubernetes** | K8s manages containers. Helvilette manages systemd services. K8s cannot self-heal its own control plane. Helvilette can. |
| **SaltStack** | Replaces Ansible entirely with its own language. Helvilette preserves Ansible. |
| **Nomad** | Requires HCL, Consul, and its own bootstrap process. Still needs Ansible to bootstrap Nomad. |

### Pros

1. **Zero learning curve** — if you know Ansible + K8s YAML, you know Helvilette
2. **Zero SSH exposure** — pull-based eliminates the largest attack vector in configuration management
3. **Zero vendor lock-in** — remove Helvilette and you still have working Ansible playbooks + Git repos
4. **AI-ready** — helvilette.yml uses K8s + Ansible conventions that are massively represented in LLM training data; any AI can generate valid configs immediately
5. **Lightweight** — single Go binary (~20MB RAM), runs on Raspberry Pi (ARM64)
6. **GitOps-native from day one** — not bolted on like Puppet r10k/Code Manager
7. **Self-reinforcing loop** — Helvilette installs monitoring (Alloy), then monitors itself through it

### Cons

1. **Nascent project** — solo developer, no production users yet beyond author's own infrastructure
2. **Ansible dependency** — requires Ansible installed on every agent node
3. **No persistence layer yet** — Othela currently holds state in memory (SQLite planned)
4. **No authentication yet** — agent ↔ Othela communication currently unauthenticated
5. **Pull-based latency** — changes propagate at poll interval speed, not instantly (webhook mitigates)

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Ansible execution failure on agent | Structured error reporting, `PlaybookFailBackOff` with exponential backoff |
| Othela control plane goes down | Agents continue running last known config; Othela is a "post office" not the executor |
| Git server unavailable | Local repo cache; `GitPullBackOff` with retry |
| Agent compromised | Agent only executes playbooks from registered repos; no arbitrary code execution |
| Config drift during poll gap | Adjustable poll interval; webhook for instant notification |

### Security Considerations

- **SSH elimination** — the primary security improvement. No inbound ports required on managed nodes.
- **Outbound-only connections** — agents initiate all communication. Firewall-friendly.
- **No credential storage on control plane** — unlike AWX, Othela does not store SSH keys. There are no SSH keys.
- **Git-based audit trail** — all configuration changes are Git commits with full history.
- **Planned: mTLS** — mutual TLS between agent and Othela for authenticated communication.
- **Planned: API key authentication** — pre-shared token or API key for agent registration.

## Future Milestones

1. **Drift Detection** — `ansible-playbook --check` mode for detecting without correcting
2. **Dashboard UI** — Wails v2 + Vue/Svelte with Death Stranding-inspired industrial aesthetic
3. **Rolling Update Strategy** — sequential node updates with health checks between each
4. **Canary Deployments** — apply to 1 node first, verify probes, then roll out
5. **Namespace Support** — environment separation (prod/staging/dev)
6. **RBAC** — control who can push playbooks to which namespaces
7. **CNCF Sandbox Application** — when community adoption and governance justify it

## Implementation Details

### Testing Plan

- **Unit tests**: Playbook loader (87.8% coverage), systemd types, agent configuration
- **E2E tests**: Docker Compose cluster (1 Othela + 3 Agents + Gitea + git-seeder)
- **Integration**: Agent → Othela → Git repo → Ansible execution → Report cycle verified

### Update/Rollback Compatibility

- Rollback via `git revert` — agents pull previous version on next poll
- Planned: `helvilette.io/rollback-to` annotation for targeted version pinning
- Backward compatibility maintained through `omitempty` JSON fields in Job struct

### Scalability

- Pull-based model scales naturally — each agent independently polls Othela
- Git repo caching reduces redundant clones
- Poll interval configurable per agent
- Webhook support eliminates polling overhead for immediate updates

### Implementation Phases/History

| Phase | Status | Description |
|---|---|---|
| 1.0 Living Skeleton | ✅ Complete | First E2E: Agent → Poll → Othela → Execute → Report |
| 1.5 Ephemeral Lab | ✅ Complete | Docker Compose cluster (1 Othela + 3 Agents) |
| 1.6 K8s-Style Config | ✅ Complete | Cobra CLI + YAML config (kubelet-style) |
| 1.7 Git Server Mock | ✅ Complete | Gitea + git-seeder for E2E tests |
| 2.0 GitOps Distribution | ✅ Complete | Agent git clone/pull, reference-based jobs |
| 2.1 helvilette.yml Spec | ✅ Designed | Declarative config format with K8s API surface |
| 3.0 Persistence | 🔲 Planned | SQLite for Othela state |
| 3.1 Authentication | 🔲 Planned | API key / mTLS for agent ↔ Othela |
| 3.2 Node Targeting | 🔲 Planned | Label-based job routing via nodeSelector |
| 3.3 Drift Detection | 🔲 Planned | `--check` mode, DriftDetected events |
| 3.4 Health Probes | 🔲 Planned | livenessProbe / readinessProbe for systemd services |
| 4.0 Dashboard UI | 🔲 Planned | Wails v2 web interface |
| 5.0 Production Readiness | 🔲 Planned | Graceful shutdown, health endpoints, systemd service files |
