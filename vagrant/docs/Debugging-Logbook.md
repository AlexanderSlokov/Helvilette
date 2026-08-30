# Debugging Logbook: First Light Operational Issues

This logbook details the operational issues and debugging experience during the First Light manual test. It focuses on how Othela's current implementation behaves contrary to expected operational flows.

## Stale Fleet Repo Flag
The documentation and E2E configuration (`docker-compose.e2e.yaml`) imply the existence of a `--fleet-repo` flag for GitOps operations. However, attempting to run the Othela CLI with this flag results in an immediate `unknown flag: --fleet-repo` failure. The control plane currently only supports a local `--playbook-dir`. This discrepancy misleads operators setting up a GitOps pipeline and breaks the provided E2E test scripts.

## Silent Failures in Playbook Loader
When falling back to the supported `--playbook-dir` mechanism, the operator experience degrades due to silent failures in the `filepath.Walk` playbook loader:
1. A valid manifest was provisioned at `/var/lib/helvilette/playbooks/baseline/helvilette.yml`.
2. Directory permissions (`0700`) and ownership (`helvilette:helvilette`) were perfectly aligned with the Othela service process.
3. Upon startup, Othela logged `count: 0` and `scan complete` without a single `Warn` log to indicate parsing errors, access denials, or skipped directories.
4. The lack of trace/debug logging around files that are evaluated but silently skipped by `loader.go` makes it nearly impossible for an operator to diagnose why a fleet deployment is not dispatching to agents.

## Remediation Plan
GitHub issues have been fired to address these specific pain points:
- Issue #33: Implement or remove the ghost `--fleet-repo` flag in E2E.
- Issue #34: Fix the playbook loader's silent skipping behavior and add appropriate debug trace logs for skipped files.
