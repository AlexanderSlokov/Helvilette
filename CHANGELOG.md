# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

* The agent logs its resolved configuration at startup under the message
  `effective configuration`, naming the source of every value — `config-file`,
  `env(NODE_ID)`, `cli(--othela-url)`, `default`, or `default(hostname)`. Individual labels
  are reported per key. A node's behaviour can now be explained from its own logs, without
  reconstructing the precedence rules from its systemd unit and container environment.
  ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))

* New `--print-config` flag resolves the configuration, prints each value with its source,
  and exits without starting the agent. Useful for day-0 bring-up and for validating a
  config file in CI. ([#11](https://github.com/AlexanderSlokov/Helvilette/issues/11))

### Changed

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
