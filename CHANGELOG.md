# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

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
