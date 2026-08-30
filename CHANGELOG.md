# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
  `apiVersion: helvilette.io/v1alpha1` and `kind: PlaybookDeployment`, replacing the previous
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
