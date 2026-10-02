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
      Resolved: PlaybookContent removed from pkg/types/types.go, all fallback/mock inline-content
      jobs deleted from server.go, and the agent now rejects jobs with neither RepoURL nor
      PlaybookPath. Tests rewritten to exercise the reference model. Issue #25.
- [x] Agent Git Package (pkg/git): Implement pkg/git/cache.go and pkg/git/clone.go.
- [x] Agent Execution Logic Update: Update ExecutePlaybook to check repo cache -> clone/pull -> run ansible-playbook from local path.
- [x] E2E/Integration Tests: Ensure Othela sends reference -> Agent successfully pulls from local Gitea and executes.

### 3.2. Node Targeting & Label-Based Routing
Distribute Jobs based on Node Labels and Registration.
- [x] pkg/manifest package: Parse helvilette.yml into Go structs.
- [x] Manifest schema identity: apiVersion helvilette.naughtian.org/v1alpha1, kind PlaybookDeployment.
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
- [ ] Bash install script (get.helvilette.naughtian.org).
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
Issue: #15. ADR: ADR-0004. Resolved.
- [x] Decide semantics for multiple matching nodeGroups: reject manifests where two
      nodeGroups carry identical nodeSelector maps at load time (breaking change).
      Rationale: conflicting extra_vars and vault-password-file values are a specification
      error, not a feature. See ADR-0004.
- [x] Add validation to pkg/manifest/validation.go (validateNodeGroupSelectorUniqueness).
      Scope: exact map equality. Subset/superset overlap deferred to v1beta1.
- [x] Fix e2e manifest: high-performance-proxies now has {role: edge-proxy, tier: high-performance}.

### 6.2. Fallback HELV_TEST_REPO_URL is dead code
Now provably unreachable, not merely suspected. `handleSync` in
`cmd/othela/server.go` falls back to `HELV_TEST_REPO_URL`, then to a hardcoded
`http://git-server:3000/helvilette/nginx-collection.git`, when
`pb.Manifest.Spec.Repo` is empty. But `validateSpec` has rejected an empty
`spec.repo` since ADR-0002, and a manifest that fails validation never becomes a
playbook, so `Spec.Repo` is never empty for anything `handleSync` iterates over.
Nothing sets the variable either: ADR-0007 removed it from the compose file, and
`cmd/othela/server.go:242` is now its only mention in the tree.
- [x] Deleted the fallback branch, the environment variable and the hardcoded URL.
      `jobFor` now reads `spec.repo` directly, with a comment recording why no
      fallback is needed. `handleSync` was 80 lines and is now four functions:
      the HTTP concerns, `jobForLabels`, `jobFor` and `injectedJob`.

### 6.3. Nested e2e manifest is outdated compared to working tree
Issue: #20, #24. ADR: ADR-0003. Resolved.
The git-server serves the committed manifest while Othela reads the working tree, so the two
disagree whenever helvilette.yml is edited without committing. Root cause is that Othela treated
a mutable directory as a versioned artifact.
- [x] Mount the playbook directory read-only in both docker-compose.e2e.yaml and the
      testcontainers suite, so nothing can write to it during a run.
- [x] Resolve playbooks on the Othela side by Git reference rather than by reading a local
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
- [x] Have Othela resolve playbooks by Git reference. Tracked jointly with 6.3. Issue: #24.

### 6.6. make e2e hardcodes a machine-specific Go SDK path
Issue: #18. Resolved.
- [x] Remove /home/stella/sdk/go1.26.1/bin from the e2e target in Makefile.
- [x] Resolve ginkgo through the module toolchain so the suite runs under the version
      pinned in go.mod.

### 6.7. Two files exceed the file size limit
AGENTS.md sets a 500-line ceiling per file. Both files that were approaching it
have now crossed it: `cmd/agent/main.go` is 819 lines and
`cmd/othela/server.go` is 520. The fleet sync work in ADR-0005 pushed the second
one over.
- [x] Split cmd/agent/main.go, 819 lines, into `config.go` (321), `agent.go` (334),
      `executor.go` (134) and `main.go` (112). `main_test.go` was split the same
      way, so a test file still sits beside what it tests. `newRootCmd()` is
      extracted, which unblocked the Agent unit-flag guard below.
- [x] Split cmd/othela/server.go, 530 lines, into `server.go` (410) and
      `fleetsync.go` (130). Every non-test file in the tree is now under the
      ceiling; the largest is server.go at 410.

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

## 8. First Light Remediation

### 8.0. Next up

ADR-0007 D2 is two pieces of work. The first shipped with the e2e rebuild; the
second has not started.

- [x] Give both components a predictable name in the journal. The units are
      `helvilette-othela.service` and `helvilette-agent.service`, each setting
      `SyslogIdentifier=`, so `journalctl -t 'helvilette*'` and
      `journalctl -u 'helvilette-*'` select the whole product without knowing
      either unit name. Asserted by the e2e suite.
- [ ] Emit real journal fields instead of a JSON blob inside `MESSAGE`. Today
      stdout reaches journald and the whole JSON line lands in one opaque field,
      so `journalctl NODE_ID=node-1` does not work and a collector needs a JSON
      parse stage. Wanted: a journald writer behind `pkg/log`, selected when
      `/run/systemd/journal/socket` exists rather than by a flag, because a flag
      is one more thing an operator has to know. Outside systemd, JSON on stdout
      stays exactly as it is.
      Why it matters: Alloy and Netdata read journald the moment they are
      installed, so this is what makes Helvilette's logs collectable without
      handing any collector a path. ADR-0007 R1, D2.
- [x] Delete `vagrant/` and rebuild the manual-test environment as a Compose
      stack of systemd-in-container nodes. ADR-0007 D1, D3, D4, D5.
      `docker-compose.e2e.yaml` is now `e2e.compose.yml` and is the only
      definition of the stack; the Ginkgo suite drives that file instead of
      restating the topology. Fixtures are tracked under `tests/fixtures/` and
      baked into the git server image at build time. The distributable images
      moved to `build/` and CI builds them.


- [x] Issue #31: Standardize Othela startup logs to structured JSON exclusively. Remove plain-text log calls from control plane.
- [x] Issue #32: Add periodic polling or webhook receiver for `--fleet-repo` in Othela to detect new commits and dispatch jobs.
      The poll loop existed but could never see a new commit: `pkg/git` resolved the
      branch through `ResolveRevision`, which matches `refs/heads/<branch>` — a ref
      that fetch never advances. `EnsureRepo` now resolves `refs/remotes/origin/<ref>`
      and hard-resets, and returns the commit SHA so a moved fleet logs at `info`.
      Webhook receiver deferred: it needs its own authentication decision. See
      ADR-0005.
- [x] Issue #33: Fix unknown flag `--fleet-repo` in E2E. The `docker-compose.e2e.yaml` uses `--fleet-repo` which Othela CLI does not actually support. Either implement the flag or update the E2E setup.
      Already fixed by commit `caab99c`, which added the flag hours after the issue
      was filed. `TestOthelaUnitFlagsExistOnTheCLI` reads the `ExecStart=` of
      `tests/images/node/helvilette-othela.service` and asserts every long flag it
      passes is registered on the CLI, so the two cannot drift apart again. It
      reads the unit rather than a compose `command:` because ADR-0007 moved the
      flags there. Verified to fail when the unit passes a flag the CLI lacks.
- [x] Issue #34: Fix playbook loader silent skip. Othela `loader.go` silently ignores playbooks (returns `count: 0`) under certain directory conditions without emitting any `Warn` logs, making debugging difficult. Add proper trace logging.
      Every path the walk declines now logs with a `skip_reason`; a near-miss
      filename such as `helvilette.yaml` warns instead of vanishing; the closing
      line carries `base_dir`, `files_examined`, `skipped`, `rejected` and
      `near_misses`, and a count of zero is a warning, not an info line. See
      ADR-0006.

- [x] Issue #37: README documented `--playbook-dir`, removed by ADR-0003, which exits
      with an error when passed, and documented none of the fleet flags. The example
      and flag table now cover `--fleet-repo`, `--fleet-branch`, `--fleet-sync-interval`,
      `--state-dir` and `--log-level`.
- [x] Issue #38: Two data races in the fleet sync path. `Server.loader` was written by
      the sync goroutine while `GetLoader` read it, and `Loader.playbooks` was rewritten
      by `Scan` while `Get`/`GetByName` read it from HTTP handlers. Both guarded;
      `go test -race ./cmd/... ./pkg/...` is clean.

### Follow-ups opened during this remediation

- [ ] `Agent.ExecutePlaybook` in `cmd/agent/executor.go` is about 110 lines,
      against the 4-20 line rule in AGENTS.md. It does four things: resolve the
      playbook path, materialise the Git repository, build and run the
      `ansible-playbook` command, and normalise the output. Splitting it was left
      out of the 6.7 file split deliberately, to keep that change mechanical and
      reviewable.

- [x] Issue #38 follow-up: CI runs a `-race` step. `make test` still does not, so
      a local run will not catch what CI does.
- [ ] Git credential support for private fleet and playbook repositories. Neither
      Othela nor the Agent can authenticate to a Git server today; both clone
      anonymously. Deliberately out of scope for ADR-0007, which brings the public
      path up first. Scope when picked up: token in an environment variable versus
      a credential file versus an SSH key, where the secret lives on the node, and
      whether Othela hands job credentials to agents or each agent holds its own.
      A public repository has to work before any of that is worth designing.
- [ ] Webhook receiver for `--fleet-repo`, so a push propagates without waiting
      out `--fleet-sync-interval`. Blocked on an authentication decision: it is an
      unauthenticated write endpoint on the control plane.
- [ ] `git clean` in the fleet cache. `EnsureRepo` hard-resets, which does not
      remove untracked files. A `helvilette.yml` left behind by an earlier commit
      would still be loaded. Deferred until observed to matter; see ADR-0005.
- [ ] Subset-overlap rejection for `nodeGroup` selectors, still deferred to
      v1beta1 by ADR-0004.
- [ ] The e2e suite asserts both agents ran a playbook, but not that the right
      playbook ran on the right node. Assert the job ID, which carries the
      manifest and nodeGroup name, rather than only that execution succeeded.
- [x] Guard the Agent's unit flags the way `TestOthelaUnitFlagsExistOnTheCLI`
      guards Othela's. `TestAgentUnitFlagsExistOnTheCLI` reads the `ExecStart=` of
      `tests/images/node/helvilette-agent.service` against `newRootCmd()`.
      Verified to fail when the unit passes a flag the binary does not define.

### Vagrant manual-test environment: replaced

Resolved by ADR-0007. Recorded here because the reasoning is worth keeping.

The environment was flaky because it had no reproducible starting state. Four
separate dependencies on host condition:

- The virtual machines never built Helvilette. `creates: /vagrant/bin/othela`
  skipped the build, and Vagrant's rsync copies the gitignored `bin/` from the
  host, so the binary under test was whatever the host last built. The build
  branch that was skipped could not have worked anyway: the playbook installed
  `golang-go` from Debian bookworm apt, far below the `go 1.25.6` in `go.mod`.
- Gitea was bootstrapped by hand through its web UI, so the git server's state
  depended on whether someone did the clicks and did them the same way.
- The fixture lived in a gitignored directory, so two machines were not
  necessarily running the same manifest. It also declared `nodeSelector: {}`,
  which ADR-0004 rejects at load time, so its playbook was never dispatched.
- Provisioning order guaranteed a broken first boot: Othela started in play 2
  pointed at a repository that play 4 had not yet created.

Every one of those is gone in the Compose stack. The fixture survives as
`tests/fixtures/baseline/`, now with a real selector.
