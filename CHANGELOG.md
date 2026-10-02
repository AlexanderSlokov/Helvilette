# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

* **Helvilette emits native journal fields when it runs as a systemd unit.**
  Every key becomes a real journald field, so an operator can address them:

  ```
  journalctl NODE_ID=node-1
  journalctl COMPONENT=playbook-loader
  journalctl -t 'helvilette*' -o cat     # prose, not a JSON document
  ```

  Previously the whole JSON line landed in one opaque `MESSAGE`, so no field query
  matched anything and a log collector needed a JSON parse stage before any of it
  was usable. Alloy and Netdata read journald the moment they are installed, so
  this is what makes Helvilette's logs collectable without handing any collector a
  path. Implemented in `pkg/log/journald.go` on `journal.Send` from
  `github.com/coreos/go-systemd/v22`, which was already a direct dependency — no
  new one was added. ([ADR-0007](docs/informations/ADRs/ADR-0007.md) D2)

  There is no `JSON` field duplicating the original line, unlike zerolog's own
  journald writer: `journalctl -o json` reconstructs the record from the fields,
  so keeping one would only double the volume.

* `pkg/log.IdentifierFor` derives a component's `SYSLOG_IDENTIFIER` from its
  binary name. A native journal send does not inherit the unit's
  `SyslogIdentifier=` — that applies to the stream transport — so the binary has to
  send its own, and `TestOthelaUnitIdentifierMatchesTheBinary` and its Agent
  counterpart assert the unit and the binary have not drifted apart. Without that
  the `journalctl -t 'helvilette*'` shipped in the previous release would have
  silently stopped finding anything.

### Changed

* **Behaviour outside systemd is unchanged**, and the rule for choosing is now the
  one systemd documents. The sink is selected by `JOURNAL_STREAM`, which systemd
  sets only for its own units, not by the presence of
  `/run/systemd/journal/socket`: that socket also exists when an operator runs the
  binary by hand, where upgrading to journald would swallow their terminal output.
  Precedence is journald, then `HELVILETTE_DEV=1` console, then JSON on stdout.
  ADR-0007 D2 is amended in place to record the correction.

* `pkg/log` now formats console output *behind* the swap point rather than in
  front of it, so `SetOutput` is the single seam for every destination. Existing
  tests that capture logs are unaffected.

### Removed

* 42 modules dropped from `go.mod` by `go mod tidy`. The e2e suite stopped using
  `testcontainers-go` when it moved to driving `e2e.compose.yml` directly, and its
  dependency tree had been carried since.


### Removed

* **Unreachable test scaffolding in Othela's dispatch path.** `handleSync` fell
  back to `HELV_TEST_REPO_URL` and then to a hardcoded
  `http://git-server:3000/helvilette/nginx-collection.git` when a manifest's
  `spec.repo` was empty. `validateSpec` has rejected an empty `spec.repo` since
  ADR-0002, and a manifest that fails validation never becomes a playbook, so the
  branch could not be reached. Nothing set the variable either once the e2e
  rebuild dropped it from the compose file. (BACKLOG 6.2)

### Changed

* **Two files that exceeded the 500-line ceiling in AGENTS.md are split by
  responsibility.** `cmd/agent/main.go`, 819 lines, becomes `config.go`,
  `agent.go`, `executor.go` and `main.go`; its tests are split the same way so a
  test file still sits beside what it tests. `cmd/othela/server.go`, 530 lines,
  becomes `server.go` and `fleetsync.go`. No behaviour change: the test count is
  unchanged and the race detector stays clean. The largest non-test file in the
  tree is now 410 lines. (BACKLOG 6.7)

* `handleSync` was 80 lines against a 4-20 line rule. It is now the HTTP concerns
  plus `jobForLabels`, `jobFor` and `injectedJob`, the last of which documents
  why a server built by `NewServerWithJob` stops serving its fixture as soon as a
  real manifest loads.

### Added

* `TestAgentUnitFlagsExistOnTheCLI` guards the Agent's systemd unit against the
  flag drift that caused issue #33, the way `TestOthelaUnitFlagsExistOnTheCLI`
  already guarded Othela's. It needed `newRootCmd()` to be separated from `main()`
  first: a `cobra.Command` built inside `main` cannot be reached from a test.
  Both guards are verified to fail when a unit passes a flag the binary lacks.


### Changed (Breaking)

* **The Vagrant manual-test environment is deleted and replaced by a Compose stack
  of systemd-in-container nodes.** `docker-compose.e2e.yaml` is renamed
  `e2e.compose.yml` and is now the only definition of the stack: the Ginkgo suite
  brings that file up rather than declaring the same four services a second time.
  The previous split had to be edited in pairs, and issue #33 is what that looks
  like when it fails.

  Othela and the agents run under `geerlingguy/docker-ubuntu2404-ansible` with
  systemd as PID 1, as `helvilette-othela.service` and `helvilette-agent.service`.
  Managing systemd units is what Helvilette does, so a foreground-process
  container could not exercise it — nor journald, which is what makes logs
  collectable without configuring a path.

  The git server image commits three repositories at build time: `fleet` (the
  manifests Othela reads) and `nginx-collection` and `baseline` (the playbooks
  agents run). Fleet and playbook repositories are now separate, as ADR-0003
  intended; the previous fixture used one repository for both, so the separation
  was never exercised.

  `Dockerfile.othela` and `Dockerfile.agent` move to `build/othela/Dockerfile` and
  `build/agent/Dockerfile`. `Dockerfile.gitserver` is replaced by
  `tests/images/gitserver/Dockerfile`. Fixtures move from `tests/e2e/data/playbooks/`
  to `tests/fixtures/` and are tracked. See
  [ADR-0007](docs/informations/ADRs/ADR-0007.md).

### Fixed

* **The E2E suite had been failing since 2026-09-07** and nothing acted on it. It
  asserted on `"[DEBUG] Node agent-02 has labels"`, a plain-text log line that the
  zerolog migration in the same release replaced with JSON, so the assertion could
  never match again. The suite is rewritten around the compose stack and no longer
  greps for log prose where an API answer will do.

* **Nothing built the distributable images.** Their only consumer was the e2e
  stack, which no longer uses them. `make images` builds both and CI runs it as
  its own job, because an image nothing builds stops being buildable without
  anyone noticing — which is how the Vagrant environment died.

* **CI never ran the race detector.** Added as its own step. Two data races in the
  fleet sync path had lived undetected; see #38.

* `tests/fixtures/baseline/` (recovered from the deleted `vagrant/baseline-repo/`,
  which was gitignored) declared `nodeSelector: {}`. ADR-0004 rejects an empty
  selector at load time because it matches no node while reading as "matches every
  node", so that playbook was never dispatched to anything. It now carries a real
  selector.

### Added

* `make journal` follows Helvilette's units on a node, with `NODE=` selecting
  which. Per-unit output lives in journald rather than the container log now that
  the nodes run systemd, so `make logs` shows the boot transcript and this shows
  Helvilette.

* Both units set a `helvilette-` `SyslogIdentifier`, so `journalctl -t 'helvilette*'`
  selects the whole product without knowing either unit name. This is the first
  half of ADR-0007 D2; emitting native journal fields rather than JSON inside
  `MESSAGE` is the next item in BACKLOG section 8.


### Fixed

* **Othela never picked up commits pushed to `--fleet-repo` after startup.** The
  poll loop ran on schedule, but `pkg/git` resolved the branch with go-git's
  `ResolveRevision`, which expands a bare name through `refs/heads/<branch>`
  before `refs/remotes/<branch>`. A fetch advances only `refs/remotes/origin/*`,
  so `refs/heads/main` stayed at the commit the clone landed on and every poll
  checked out that same commit while logging success. `EnsureRepo` now resolves
  `refs/remotes/origin/<ref>`, then `refs/tags/<ref>`, then `<ref>` as a commit
  SHA, and converges the worktree with a hard reset. It returns the resolved SHA,
  which Othela logs as `fleet_commit` / `previous_commit`. The same call path is
  used by the Agent, so a job pinned to a branch also ran a stale commit.
  ([#32](https://github.com/AlexanderSlokov/Helvilette/issues/32), ADR-0005)

* **The playbook loader gave no account of what it skipped.** `{"count":0}` named
  neither the directory scanned nor how many files were examined, and a manifest
  named `helvilette.yaml` was passed over in silence. Every path the walk declines
  now logs with a `skip_reason` (`hidden_dir`, `not_a_manifest`, `parse_rejected`),
  a near-miss filename warns naming both what was found and what was expected, and
  the closing line carries `base_dir`, `files_examined`, `skipped`, `rejected` and
  `near_misses`. A count of zero is logged at WARN, not INFO: it means no node will
  be dispatched anything.
  ([#34](https://github.com/AlexanderSlokov/Helvilette/issues/34), ADR-0006)

* **Two data races in the fleet sync path.** `Server.loader` was written by the
  sync goroutine while `GetLoader` read it, and `Loader.playbooks` was rewritten by
  `Scan` while `Get` / `GetByName` read it from HTTP handlers. Both are now guarded.
  `go test -race ./cmd/... ./pkg/...` is clean.

* **The fleet sync goroutine outlived graceful shutdown.** It had no exit. It now
  takes a `context.Context`, cancelled during drain alongside `SetReady(false)`.

* **A half-finished clone of the fleet repository never self-healed.** A cache
  directory that existed but would not open as a repository failed on every
  subsequent poll. It is now discarded and cloned again — on an open failure only,
  never on a fetch failure, so a network outage leaves the existing checkout intact.

* **README documented `--playbook-dir`**, a flag removed in ADR-0003 that exits
  with an error when passed, and documented none of the fleet flags.

### Changed (Breaking)

* Othela now resolves playbooks (manifests) exclusively by Git reference. The `--playbook-dir` flag has been removed and replaced by `--fleet-repo` (required) and `--fleet-branch`. This completes the GitOps transition described in ADR-0003 and Issue #24.

* `git.EnsureRepo` returns `(string, error)` instead of `error`; the string is the
  resolved commit SHA. `Server.StartFleetSync` takes a `context.Context` and a
  `FleetSyncConfig` in place of four positional arguments. Both call sites are
  updated. (ADR-0005)

### Added

* `pkg/log.SetOutput(io.Writer) io.Writer` redirects all log output and returns the
  previous destination. Log lines are part of the loader's contract now, so tests
  have to be able to read them back.

* Tests for `pkg/git`, which previously had none — including
  `TestEnsureRepo_PicksUpCommitsPushedAfterClone`, the regression test for #32,
  which fails against the previous implementation.

* `TestE2EComposeFlagsExistOnTheCLI` reads the `othela` service's command list out
  of `docker-compose.e2e.yaml` and asserts every long flag it passes is registered
  on the CLI. This is the check whose absence let
  [#33](https://github.com/AlexanderSlokov/Helvilette/issues/33) ship; #33 itself
  was already fixed by commit `caab99c`.

* **BREAKING — API group domain migration.** The manifest `apiVersion` group changed from `helvilette.io` to `helvilette.naughtian.org`. All manifests must use `apiVersion: helvilette.naughtian.org/v1alpha1`. The previous domain was never registered to this project; the new group uses a subdomain of the project-owned `naughtian.org` domain, which is the strictly correct convention per ADR-0002.

### Changed

* **Othela logging migrated from stdlib `log` to structured JSON (zerolog).**
  All `log.Printf` / `log.Fatalf` calls in `cmd/othela/` have been replaced with
  structured zerolog calls via `helvilette/pkg/log`. Log output is now exclusively
  JSON in production (or human-readable when `HELVILETTE_DEV=1`), consistent with
  the Agent. The `--log-level` flag now connects to zerolog global level filtering
  instead of only toggling a `debugMode` boolean. The `DebugMode` field has been
  removed from `ServerConfig`, and the `SetDebug()` method has been removed from
  `Server`. Error messages now include actionable context (e.g. which flag to check,
  which directory to verify).
  ([#31](https://github.com/AlexanderSlokov/Helvilette/issues/31))

### Added

* `helvilette.yml` is now validated when it is loaded. `apiVersion` and `kind` must match
  exactly, and `metadata.name`, `spec.repo`, `spec.playbook`, a non-empty `spec.nodeGroups`,
  and a non-empty `nodeSelector` on every group are required. Each rejection names the
  offending field, its value, and the expected shape. Previously a manifest with a stale
  schema or a misspelled key — `nodegroups` for `nodeGroups` — unmarshalled cleanly into an
  empty manifest, matched no node, and left every agent receiving `204 No Content` with no
  error and no log line pointing at the file. A rejected manifest is now logged at WARN
  stating that its playbook will not be dispatched.
  ([#13](https://github.com/AlexanderSlokov/Helvilette/issues/13))

* The agent logs its resolved configuration at startup under the message
  `effective configuration`, naming the source of every value — `config-file`,
  `env(NODE_ID)`, `cli(--othela-url)`, `default`, or `default(hostname)`. Individual labels
  are reported per key. A node's behaviour can now be explained from its own logs, without
  reconstructing the precedence rules from its systemd unit and container environment.
  ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))

* New `--print-config` flag resolves the configuration, prints each value with its source,
  and exits without starting the agent. Useful for day-0 bring-up and for validating a
  config file in CI. ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))

* New `make` targets for the development loop: `fmt-check` verifies gofmt without rewriting
  files and is the same check CI runs, and `clean-e2e` tears down the e2e stack and removes
  the runtime state it writes to `tests/e2e/data` and `data/`. Both are documented in the
  README under Development Setup.
  ([#17](https://github.com/AlexanderSlokov/Helvilette/issues/17))

### Changed

* **BREAKING — Manifests with overlapping `nodeGroup` selectors are rejected.** Previously,
  if multiple node groups had identical label selectors (e.g., both targeting `role: edge-proxy`),
  the agent would only apply the first one and silently ignore the rest. This was a source of
  confusion as conflicting `extra_vars` or `vault-password-file` values would be ignored without
  any indication. Now, validation will fail at load time, requiring the operator to provide
  distinct selectors for each group. See [ADR-0004](docs/informations/ADRs/ADR-0004.md).
  ([#15](https://github.com/AlexanderSlokov/Helvilette/issues/15))
* **BREAKING — `Job.PlaybookContent` removed from the wire format.** The field carried an
  inline Ansible playbook and was the original delivery mechanism before GitOps references
  (`RepoURL`, `PlaybookPath`, `Version`) replaced it. No production code path read or wrote
  it; only tests and fallback constructors kept it alive. The agent now rejects a job that
  carries neither `RepoURL` nor `PlaybookPath` with a clear error instead of silently writing
  empty content to disk. All mock/fallback inline-content jobs in Othela have been deleted;
  dispatch is driven entirely by manifest matching.
  ([#25](https://github.com/AlexanderSlokov/Helvilette/issues/25))

* **BREAKING — `--data-dir` removed, replaced by `--playbook-dir` and `--state-dir`.** The old
  flag named the directory Othela loads playbooks from, and also received the SQLite database at
  `{data-dir}/server/db/state.db`. Read-only input and read-write state therefore shared a
  directory, and in the e2e stack that directory is bind-mounted from inside the Go module tree.
  Othela running as root wrote `tests/e2e/data/playbooks/server` back to the host as `root:root`
  mode 750, and `go vet ./...` then failed with `permission denied` before compiling anything.
  The same conflation is behind the two-sources-of-truth defect in BACKLOG 6.3, where the
  e2e git-server serves the committed manifest while Othela reads the working tree.

  `--playbook-dir` is read-only input, defaulting to `helvilette/othela/data/playbooks` — which
  also corrects the misspelling in the previous default. `--state-dir` is writable, defaulting to
  `/var/lib/helvilette/othela`, the FHS location the systemd units in BACKLOG 3.5 will need. The
  database moves to `{state-dir}/db/state.db`.

  Passing `--data-dir` now exits with an error naming both replacements and their defaults,
  rather than being silently ignored. No deprecated alias is provided: the flag designated the
  playbook directory while also holding state, so mapping it onto either replacement would be
  wrong half the time. Rationale in
  [ADR-0003](docs/informations/ADRs/ADR-0003.md).
  ([#20](https://github.com/AlexanderSlokov/Helvilette/issues/20))

* **BREAKING — manifest schema identity.** `helvilette.yml` now requires
  `apiVersion: helvilette.naughtian.org/v1alpha1` and `kind: PlaybookDeployment`, replacing the previous
  `apps/v1` / `Cluster`. `apps/v1` is an occupied Kubernetes in-tree group and the domain-less
  form is a 1.x holdover, not a pattern for new groups; projects file their own kinds under a
  domain they own, as k3s does with `k3s.cattle.io` and `helm.cattle.io`. `v1alpha1` reflects
  that `spec.vault` and `nodeGroups[].probes` are still declared but unparsed. `Cluster` became
  `PlaybookDeployment` because the file declares a playbook rolled out to node groups, not a
  cluster.

  **Action required:** update `apiVersion` and `kind` in every `helvilette.yml`. A manifest on
  the old identity is now rejected with a message naming both the found and expected values,
  rather than silently deploying to nobody. See
  [ADR-0002](docs/informations/ADRs/ADR-0002.md).
  ([#1](https://github.com/AlexanderSlokov/Helvilette/issues/1),
  [#13](https://github.com/AlexanderSlokov/Helvilette/issues/13))

* **BREAKING — `nodeID` now defaults to the machine hostname** instead of the static
  `agent-01`. A static default meant every node that reached it registered under the same
  identity; this is what turned the config-key bug in #8 from one misconfigured node into a
  fleet-wide identity collision. If the hostname cannot be determined the agent falls back
  to `agent-unknown` and logs a warning.

  **Action required:** any node that relied on the implicit `agent-01` — rather than setting
  `nodeID` in its config file, `NODE_ID`, or `--node-id` — will register under a new identity
  after upgrading. Set `nodeID` explicitly to pin it.
  ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))

* `LoadConfig` now returns a third value, `ConfigProvenance`, recording which source supplied
  each field. This is a source-level change for anyone calling it directly.
  ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))


* **BREAKING — Agent configuration precedence.** The YAML config file now outranks
  environment variables. The effective order is **CLI flags > YAML config > environment
  variables > defaults**, which is what the README has always documented; the
  implementation previously applied the file *before* the environment, so an ambient
  `OTHELA_URL`, `NODE_ID`, `POLL_INTERVAL`, `WORKSPACE_DIR` or `AGENT_LABELS` silently
  overrode the file.

  **Action required:** any deployment that relies on an environment variable to override a
  value set in `agent.yaml` will change behaviour after upgrading — the file now wins. Move
  such overrides to CLI flags, which remain the highest-priority source, or remove the value
  from the config file. See [ADR-0001](docs/informations/ADRs/ADR-0001.md) for the rationale.

* Labels from the config file now merge per key with labels from the environment, instead of
  replacing the whole set. A key set only in `AGENT_LABELS` survives unless the file sets that
  same key. This keeps label handling consistent across all three sources.
  ([#9](https://github.com/AlexanderSlokov/Helvilette/issues/9))

### Fixed

* The e2e git-server no longer installs git at container start. It ran
  `apk add --no-cache git git-daemon` and the suite waited for its `Ready to rumble` log line,
  so every run depended on a package download completing inside the startup deadline. It was
  observed failing under load and passing on an idle host. Git is now baked into
  `Dockerfile.gitserver`, and every readiness wait sets an explicit `WithStartupTimeout` instead
  of relying on the default. ([#20](https://github.com/AlexanderSlokov/Helvilette/issues/20))

* Removed `cmd/othela/cmd`, an unused `cobra init` scaffold. Nothing imported it, its `Run` only
  printed "This is where the server startup logic will go", and it declared a third `--data-dir`
  flag that would have survived the removal above and misled anyone grepping for it.
  ([#20](https://github.com/AlexanderSlokov/Helvilette/issues/20))

* Host-side Go tooling no longer breaks after running the e2e stack. Othela bind-mounts
  `tests/e2e/data` and ran as root, so it wrote its SQLite state back to the host as
  `tests/e2e/data/playbooks/server`, owned by `root:root` mode 750. Because that path sits
  inside the Go module tree, `go vet ./...` and `go list ./...` failed with
  `permission denied` before compiling anything — including the exact `go vet ./...` that CI
  runs. CI stayed green only because a fresh checkout never has the directory. Othela now
  runs as the host UID, the path is gitignored, and `make clean-e2e` removes leftovers from
  older stacks using a throwaway container rather than requiring sudo. Agents deliberately
  remain root: they apt-install inside the container and write only to gitignored `./data`.
  ([#17](https://github.com/AlexanderSlokov/Helvilette/issues/17))

* `make test` no longer hangs. It ran `go test ./...`, which pulled in the ginkgo e2e suite
  and did not complete within 120s without a running Docker stack. Unit-test targets are now
  scoped to `./cmd/... ./pkg/...` and end-to-end stays in `make e2e`; a full unit run takes
  0.9s. ([#17](https://github.com/AlexanderSlokov/Helvilette/issues/17))

* `make up`, `make down`, and `make logs` now work. All three called `docker compose` with no
  `-f`, and the repo has no default compose file, so Docker Compose had nothing to load.
  ([#17](https://github.com/AlexanderSlokov/Helvilette/issues/17))

* CI now runs the agent's tests. The pipeline ran `go test ./cmd/othela/...` and
  `./pkg/...` as separate steps, which never executed `./cmd/agent/...` despite its 552-line
  test file, and ran `./pkg/storage/...` twice. Collapsed into one step covering `./cmd/...`
  and `./pkg/...`. ([#17](https://github.com/AlexanderSlokov/Helvilette/issues/17))

* `make e2e` no longer hardcodes a machine-specific Go SDK path. The target prepended
  `/home/stella/sdk/go1.26.1/bin` to `PATH`, which resolved to nothing on any other machine
  and could silently run the suite under a toolchain that did not match `go.mod`. Ginkgo is
  now invoked via `go run github.com/onsi/ginkgo/v2/ginkgo`, which uses the version pinned
  in `go.mod` and works on any machine with a Go toolchain.
  ([#18](https://github.com/AlexanderSlokov/Helvilette/issues/18))

* Unrecognised keys in the agent config file are now rejected at startup instead of being
  silently ignored. A misspelled key previously left the agent quietly running on defaults —
  polling `http://localhost:8080/api/v1` and registering as `agent-01`.
  ([#8](https://github.com/AlexanderSlokov/Helvilette/issues/8))

* Corrected the YAML config example in the README, which used `otherlaUrl` and `nodeId`. The
  parser reads `othelaURL` and `nodeID`; copying the example produced a default-configured
  agent. The example now also shows `workspaceDir`.
  ([#8](https://github.com/AlexanderSlokov/Helvilette/issues/8))

## [0.1.0]

Initial release.
