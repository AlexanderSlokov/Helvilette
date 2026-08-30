# Helvilette: AI Agent Backlog & Roadmap

This document outlines the roadmap and backlog for AI Agents or contributors participating in the project's development.

## Current State (Completed)
- BDD E2E Testing Framework (Ginkgo & Gomega).
- Living Skeleton E2E (Agent pull & run Ansible).
- Server and Agent structure.
- Ephemeral Testing Environment (Docker Compose with 1 Server + 3 Agents).
- K8s-Style Configuration (Cobra CLI, YAML parser for Agent config).
- GitOps Playbook Distribution (Agent automatically clones/pulls repository and runs playbook from local Git).
- Node Targeting & Label-Based Routing (Agent registration, label matching, manifest parsing, extra_vars execution).

---

## 3. High Priority Backlog (Must-Have for next Demo / Release)

These items are core to the Pivot direction and must be completed first.

### 3.1. Phase 2: GitOps Playbook Distribution (Agent Clone/Pull)
Switch from Othela sending PlaybookContent to sending References for Agent to clone from Git.
- [x] Job Struct Update: Update Job model with RepoURL, PlaybookPath, Version, and remove PlaybookContent.
      Correction: the removal did not land. PlaybookContent is still declared in pkg/types/types.go
      and referenced only by tests. Tracked in issue #25.
- [x] Agent Git Package (pkg/git): Implement pkg/git/cache.go and pkg/git/clone.go.
- [x] Agent Execution Logic Update: Update ExecutePlaybook to check repo cache -> clone/pull -> run ansible-playbook from local path.
- [x] E2E/Integration Tests: Ensure Othela sends reference -> Agent successfully pulls from local Gitea and executes.

### 3.2. Node Targeting & Label-Based Routing
Distribute Jobs based on Node Labels and Registration.
- [x] pkg/manifest package: Parse helvilette.yml into Go structs.
- [x] Manifest schema identity: apiVersion helvilette.io/v1alpha1, kind PlaybookDeployment.
- [x] Manifest validation: Verify apiVersion, kind, and required fields upon loading. Reject invalid manifests with clear messages stating the invalid field, its value, and expected shape.
- [x] Agent labels config: Add Labels map[string]string to AgentConfiguration (CLI --labels, YAML config, ENV AGENT_LABELS).
- [x] Node Registration API: POST /api/v1/nodes/register. Agent sends nodeID and labels, Othela saves to registry.
- [x] Othela dispatcher update: handleSync reads labels from registry, matches with nodeSelector from manifest, returns the correct job and extra_vars.
- [x] Agent ExtraVars execution: Write extra_vars to a JSON file and append -e @file to ansible-playbook command.
- [x] Job struct update: Add ExtraVars map[string]string to pkg/types.Job.
- [x] Unit tests: Parser pkg/manifest and nodeSelector matching.
- [x] E2E update: Agent matching labels receives job, unmatched agent receives 204 No Content.
- [x] Othela Debug Mode: Add --log-level=debug flag, hide polling log from INFO level.

### 3.3. Persistence Layer for Othela (SQLite)
Currently, Othela stores data in memory. A database is required to record history.
- [x] Integrate SQLite driver (mattn/go-sqlite3, similar to k3s/kine).
- [x] Separate storage interface (pkg/storage): NodeStore, ReportStore.
- [x] Implement in-memory adapter (pkg/storage/memory.go).
- [x] Implement SQLite adapter (pkg/storage/sqlite.go) for Node Registry and Execution Reports.
- [x] Inject SQLite into Othela via ServerConfig. DB path is now {state-dir}/db/state.db,
      changed from {data-dir}/server/db/state.db by ADR-0003 and issue #20.
- [ ] Implement tables/models for Job History (record which job was sent to which agent, and when).
- [ ] Design schema to store the previous run's state on Othela. See issue #22: this is node status,
      not a job log, and the two are different. reports.reported_at records when Othela received a
      report, not when the node observed it, and types.Report carries no node-side timestamp at all.
      Othela must be able to answer known, known-but-stale, and unknown.
- [ ] Job state must reside in SQLite, not in Othela's RAM. Ensure Othela can survive a mid-job restart (hot-patch requirement).
- [ ] Ghost/orphan detection: Othela must detect ghosts (nodes in inventory but not existing) and orphans (nodes running but not in inventory).

### 3.4. Enroll Token & Agent Identity Lifecycle
Agents register with Othela using a one-time enroll token and receive a long-term identity.
- [ ] One-time enroll token: Othela generates token, agent calls home to register. Implement one-time use Edge Key for identity attachment.
- [ ] Othela endpoint middleware to verify token for all Agent APIs.
- [ ] Agent configures and sends Authorization header with the received token after enrollment.
- [ ] Revoke and rotate agent identity after enrollment.
- [ ] Handle stolen nodes: mechanism to revoke identity from Othela.

### 3.5. Working with systemd (Agent runtime)
Helvilette uses systemd as its runtime to interface with the OS.
- [ ] Systemd unit files: othela.service, helvilette-agent.service.
- [ ] Configure Restart=on-failure, RestartSec, StartLimitBurst in the unit file.
- [ ] Agent writes last_run_summary.json to the node's disk, enabling h8e status to run even if Othela is down.
- [ ] Validate proper agent operation after systemd restart, stop, reload.
- [ ] Ensure journalctl -u helvilette provides sufficient logs for diagnostics on the node.

### 3.6. Reconciliation Loop (Drift Detection)
Drift detection loop: poll + diff, level-triggered.

Blocked by issues #22 and #23. Reconciliation drives observed state toward desired state, and
neither half is defined yet. Issue #22 covers observed state: the manifest has Spec but no Status,
and types.Report records an event rather than a state. Issue #23 covers desired state: Othela
resolves playbooks from a mutable directory while the agent resolves them by commit SHA, so the
comparison target moves. Kubernetes settled both before generalising the control loop; the
reasoning is written up in #22.
- [ ] Implement reconciliation loop with 3 trigger sources: Git changes (poll), periodic resync (run ansible-playbook --check --diff), and manual operator trigger.
- [ ] Cache previous check results for display purposes only.
- [ ] Add random splay/jitter when polling to prevent fleet-wide thundering herd issues.
- [ ] Enable Ansible fact cache with TTL shorter than the resync interval.
- [ ] Compare current state with desired state. If drift occurs, Agent reports a DriftDetected event to Othela.

### 3.7. Health Probes for managed services
Probes are used to detect services that are running but malfunctioning.
- [ ] Expand pkg/manifest/types.go to parse probes section from helvilette.yml.
- [ ] Support liveness probe for systemd services (HTTP get, TCP socket, Exec).
- [ ] Support gate condition (replacing "readiness") for rolling update sequencing.
- [ ] Enable yaml.Decoder.KnownFields(true) in ParseFile after probes have types.
- [ ] Agent periodically checks service health, independent of the Ansible loop.
- [ ] Implement ONESHOT + halt pattern for remediation playbooks. Ensure operator remediation requires per-service opt-in with a reason and that playbooks have been dry-run before execution.
- [ ] RestartFailureBackOff when remediation fails.

### 3.8. Structured Logging (for humans and machines)
Log rich, display poor. Store events as JSONL; display only what is necessary.
- [ ] Write Ansible callback plugin for Helvilette Agent to output events as JSON lines.
- [ ] Design output format for terminal using OX symbols: +, -, ~, ↻, ?.
- [ ] Design structured JSON schema for machine consumption.
- [ ] Parse callback events into OX symbols. Translate module names to machine changes (file path, unit name, package version).
- [ ] Output ? for tasks that are not check-safe. Never hide tasks that cannot be reliably predicted.
- [ ] Write summary.json on the node. Ensure agents do not report warnings or errors for unset configuration fields (e.g., unset log_sink).
- [ ] Send summary to Othela for fleet-wide views. Only send full JSONL when explicitly requested.
- [ ] Write a one-line summary to journald/syslog for every run.
- [ ] Implement 3 log views: h8e logs <job> (default summary), h8e logs <job> --tasks (collapsed task list), and h8e logs <job> --json (raw JSONL). Unfold failing tasks automatically.

### 3.9. Agent behavior when Othela is down
- [ ] Decide whether the agent continues to run checks when disconnected from Othela.
- [ ] If continuing: queue reports locally and flush when Othela recovers.
- [ ] If halting: log the reason in last_run_summary.json, and h8e status must indicate that Othela is unreachable.

### 3.10. Job Semantics
- [ ] Jobs must have a unique ID. Agent writes "started job X" to disk before execution.
- [ ] At-most-once semantics: on restart, agent recognizes an incomplete job and does not silently retry it.
- [ ] Handle agent self-sabotage: if a playbook disrupts network connectivity, the agent must report the completed job status once the network recovers without losing data or retrying.
- [ ] Incomplete job state: h8e status displays the paused state and incomplete job ID.

### 3.11. Preflight / Preview
- [ ] Implement preflight/preview command: h8e preview <node>.
- [ ] Score preview reliability: show N/M predictable tasks and list non-check-safe tasks.
- [ ] Add heuristic to flag destructive tasks (regex patterns for rm, dd, mkfs, etc.).
- [ ] Configurable thresholds per repo in helvilette.yml.
- [ ] Generate .previewed file bound to the node state hash and commit SHA. Reject apply if state changes.
- [ ] Use a TTL lease instead of a lock for preview/apply states.

### 3.12. h8e CLI -- Operator Experience (OX) Commands
- [ ] h8e why <node>: run on node, read local state, explain what changed, who decided it, why, and rollback commands.
- [ ] h8e pause --reason "...": pause agent on node, reason is mandatory.
- [ ] h8e freeze --reason "...": freeze entire fleet from Othela.
- [ ] h8e unfreeze: unfreeze fleet.
- [ ] h8e fleet: overview of fleet status.
- [ ] h8e apply <node|--group>: manual apply with preflight prompt.
- [ ] h8e apply --force --reason "...": escape hatch with mandatory reason.
- [ ] h8e backup / h8e restore <file>: native single-file backup and restore.
- [ ] h8e tunnel <node>: open Chisel tunnel to node with auto-timeout.
- [ ] h8e status: read last_run_summary.json on the node.
- [ ] h8e sync now: immediately trigger reconciliation loop.
- [ ] h8e uninstall: completely remove agent while leaving managed services running.

### 3.13. Production Readiness
- [x] Health check endpoints (/healthz, /readyz).
- [x] Graceful shutdown handling for Othela and Agent.
- [ ] Add automated tests for error messages to ensure they always state the next action.
- [ ] Add Testcontainers test for time-to-first-success (from installation command to first successful apply on a clean VM).

### 3.14. helvilette.yml Constraints
- [ ] Validate that helvilette.yml is optional and Othela runs with sane defaults without it.
- [ ] Ensure the repo can still be run with standard ansible-playbook when helvilette.yml is present.

---

## 4. Medium Priority Backlog (Nice-to-Have / Post-MVP)

### 4.1. Agent-Othela Protocol Versioning & Efficiency
- [ ] Explicit version numbers in all protocol messages. Support backward compatibility for at least 1 minor version.
- [ ] Implement ETag for long-poll with exponential backoff and jitter.
- [ ] Implement fail-safe poll interval if all intervals are set to 0.

### 4.2. Chisel Socket Stream
- [ ] Integrate Chisel client into agent. Implement ephemeral credentials for tunnels (e.g., closing after 5 minutes of inactivity).
- [ ] Integrate Chisel server into Othela.
- [ ] Fallback mechanism when Chisel tunnel breaks.

### 4.3. Ansible Playbook & Bash Install/Uninstall Scripts
- [ ] Bash install script (get.helvilette.io).
- [ ] Ansible playbook for bootstrap.
- [ ] Auto-generate uninstall script during installation.
- [ ] Support non-interactive script execution via INSTALL_HELVILETTE_* environment variables.
- [ ] CI test: h8e uninstall on node -> agent disappears cleanly, managed services keep running.
- [ ] CI test: h8e backup -> destroy Othela -> h8e restore <file> -> agents auto-discover and history is intact.

### 4.4. Othela Playbook / Repo Management (Multi-repo support)
- [ ] pkg/git/repo.go & watcher.go: Othela automatically syncs/tracks repositories.
- [ ] API Endpoints to register, list, and manually sync Repos (POST /api/v1/repos).

### 4.5. Webhook Triggers
- [ ] Othela listens for Webhooks (from GitHub/Gitea/GitLab) on git push.
- [ ] Invalidate cache and notify relevant Agents immediately upon trigger.

### 4.6. Vault / Secret Integration
- [ ] Expand pkg/manifest/types.go to parse vault section from helvilette.yml.
- [ ] Support exported type (read secret from Othela host ENV).
- [ ] Support hashicorp_vault type (read secret from HashiCorp Vault API).
- [ ] Agent receives vault password file path from Job and injects into ansible-playbook command.

### 4.7. Scheduled Playbook Runs
- [ ] Support Cron-like scheduling to trigger jobs from Othela.

### 4.8. README Comparison Table
- [ ] Write installation comparison table for README (AWX vs Helvilette). Include ansible-pull in the comparison.
- [ ] Add maturity markers (feature status) to README.

---

## 5. Low Priority Backlog (Features for V1.x)

### 5.1. Dashboard UI (Web)
- [ ] Node list with status badges.
- [ ] Latest Job status.
- [ ] Real-time log streaming via WebSocket.
- [ ] Playbook catalog browser.

### 5.2. Multi-tenant / Namespace Support
- [ ] Add Namespace concept for environment segregation (Dev/Staging/Prod).
- [ ] RBAC for deploy permissions.

### 5.3. Open-core Boundary
- [ ] Separate commercial code into its own repository from the beginning.
- [ ] Document the free/paid boundary commitment (API and data export must never be paywalled).

### 5.4. Contributor License Agreement (CLA)
- [ ] Set up CLA before merging the first PR from external contributors.

---

## 6. Technical Debt

### 6.1. Nodes matching multiple nodeGroups only execute the first one
Issue: #15
- [ ] Decide semantics for multiple matching nodeGroups (reject overlapping selectors, merge extra_vars, or keep first-match but log a WARN).
- [ ] Add validation or logging to pkg/manifest and handleSync.
- [ ] Fix e2e manifest to reflect the chosen semantics.

### 6.2. Fallback HELV_TEST_REPO_URL is dead code
- [ ] Remove fallback branch and HELV_TEST_REPO_URL variable from docker-compose.e2e.yaml if unused.

### 6.3. Nested e2e manifest is outdated compared to working tree
Issue: #20. ADR: ADR-0003.
The git-server serves the committed manifest while Othela reads the working tree, so the two
disagree whenever helvilette.yml is edited without committing. Root cause is that Othela treated
a mutable directory as a versioned artifact.
- [x] Mount the playbook directory read-only in both docker-compose.e2e.yaml and the
      testcontainers suite, so nothing can write to it during a run.
- [ ] Resolve playbooks on the Othela side by Git reference rather than by reading a local
      directory, matching how the agent already works after 3.1. This closes the gap fully.
      Issue: #23.

### 6.4. 12 files fail gofmt
Issue: #17
- [x] Run make fmt and create a dedicated commit.
- [x] Add gofmt check to .github/workflows/ci.yml. Added as make fmt-check, so CI and local
      runs share one definition of formatted.

### 6.5. make test includes e2e and times out
Issue: #17
- [x] Limit make test to go test ./cmd/... ./pkg/... and reserve e2e for make e2e.
- [x] Fix container-created state that broke host tooling. Othela ran as root over the
      tests/e2e/data bind mount, leaving tests/e2e/data/playbooks/server owned by root
      mode 750 inside the module tree. go vet ./... and go list ./... failed with
      permission denied before compiling. Othela now runs as the host UID, the path is
      gitignored, and make clean-e2e removes leftovers from older stacks.
- [x] Fix make up, make down, and make logs. They called docker compose with no -f, and
      the repo has no default compose file, so all three were broken.
- [ ] Add e2e job to CI, or document that e2e is a manual pre-release step. Issue: #24. Deferred
      until the startup-race fix from #21 has proven stable over several runs, and requires image
      layer caching, timeout-minutes, and resolving #18 first.

### 6.9. Othela and Agent disagree on what a playbook is
Issue: #20. ADR: ADR-0003. Partially resolved.
The agent resolves playbooks by reference (repo, path, commit SHA) and caches them, per 3.1.
Othela still reads them from a local directory. The directory is now read-only and separated
from writable state, which removes the permission and mutability hazards, but the two components
still describe the same artifact differently. Reconciliation in 3.6 needs them to agree.
- [x] Split --data-dir into --playbook-dir (read-only) and --state-dir (writable).
- [x] Move SQLite to {state-dir}/db/state.db and out of the playbook directory.
- [x] Use a named volume for state in compose, so no writable path is bind-mounted into the
      Go module tree.
- [ ] Have Othela resolve playbooks by Git reference. Tracked jointly with 6.3. Issue: #23.

### 6.6. make e2e hardcodes a machine-specific Go SDK path
Issue: #18
- [ ] Remove /home/stella/sdk/go1.26.1/bin from the e2e target in Makefile.
- [ ] Resolve ginkgo through the module toolchain so the suite runs under the version
      pinned in go.mod.

### 6.7. cmd/agent/main.go exceeds the file size limit
CLAUDE.md sets a 500-line ceiling per file. cmd/agent/main.go is 708 lines and
cmd/othela/server.go is 446. Both will grow further under sections 3.6 and 3.8.
- [ ] Split cmd/agent/main.go by responsibility (config resolution, polling loop,
      playbook execution, reporting).
- [ ] Reassess cmd/othela/server.go before it crosses 500 lines.

### 6.8. CI never ran cmd/agent tests
Resolved as part of #17. Recorded because the gap existed undetected across several
releases.
- [x] CI ran go test ./cmd/othela/... and ./pkg/... separately, which never executed
      ./cmd/agent/... despite its 552-line test file, and ran ./pkg/storage/... twice.
      Collapsed into a single step covering ./cmd/... and ./pkg/....

---

## 7. Testing & Graduation Criteria (1.0.0)

These are the rigorous testing milestones required for the 1.0.0 release.

### 7.1. k3s-ansible Graduation Test
Four failure-based runs to validate correct behavior during chaos:
- [ ] Clean run: Preflight must honestly report unpredictable tasks.
- [ ] Mid-job power failure: Agent must report incomplete state upon reboot, without silently retrying.
- [ ] Mid-fleet playbook failure: Agent halts properly, allowing the operator to identify the failing line within 60 seconds without opening the repo.
- [ ] Agent self-sabotage: Agent recovers from playbook-induced network loss and reports job results correctly.

### 7.2. Concorde & Hot-patch Test
- [ ] Implement hot-patch test: restart Othela while 50 agents are actively running jobs to ensure no jobs are lost.
- [ ] Implement Concorde test framework: run a 500-node simulated fleet to evaluate incident response time window and system stability under stress.
