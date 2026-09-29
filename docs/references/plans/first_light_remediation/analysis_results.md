# First Light remediation: what issues #32, #33 and #34 actually were

Investigated 2026-09-29, on branch `migrate_log`. The three issues were filed on
2026-08-30 after the First Light manual test and tracked in BACKLOG section 8.
Reading the code found the picture was not uniform: one was already fixed, one
was fixed in appearance only, and one was half fixed.

## Issue #33 — `docker-compose.e2e.yaml` uses a non-existent `--fleet-repo`

**Already fixed.** Commit `caab99c` added the flag on 2026-08-30 at 19:39, about
three hours after the issue was filed at 16:09. `cmd/othela/main.go` registers
`--fleet-repo`, `--fleet-branch` and `--fleet-sync-interval`, and
`cmd/othela/flags_test.go` already asserted the first of these.

What was missing was anything connecting the compose file to the CLI. The
mismatch existed for as long as it did because nothing checked. Added:
`TestE2EComposeFlagsExistOnTheCLI` reads the `othela` service's `command:` list
out of `docker-compose.e2e.yaml` and asserts every long flag it passes is
registered on `rootCmd`.

## Issue #32 — no syncing mechanism for `--fleet-repo`

**The poll loop existed; it could not work.** `Server.StartFleetSync` ticked on
`--fleet-sync-interval` and called `git.EnsureRepo`. The defect was in `pkg/git`.

`EnsureRepo` fetched, then resolved the branch with `repo.ResolveRevision("main")`:

1. A go-git clone stores the refspec `+refs/heads/*:refs/remotes/origin/*`
   (`cloneRefSpec`, default branch). Fetch advances `refs/remotes/origin/main`
   only. `refs/heads/main` never moves after the clone.
2. `ResolveRevision` expands a bare name through `plumbing.RefRevParseRules`:
   `%s`, `refs/%s`, `refs/tags/%s`, **`refs/heads/%s`**, `refs/remotes/%s`,
   `refs/remotes/%s/HEAD`. `main` matched `refs/heads/main` — the stale ref —
   before it ever reached the remote-tracking one.

Confirmed empirically before the fix was written: clone at commit A, push commit
B, fetch, `ResolveRevision("main")` returns A. Permanently. Every poll checked
out the clone-time commit and logged success.

`pkg/git` had no tests at all, which is why the e2e suite never caught it — the
suite only asserts state after the first clone, where stale and fresh agree.

The same function is called by the Agent (`cmd/agent/main.go`) to materialise a
job's repository, so a job pinned to a branch ran whatever that branch pointed
at when the Agent first cloned it.

Decision and remedy are recorded in
[ADR-0005](../../../informations/ADRs/ADR-0005.md). Webhook receiver was
considered and deferred: it is an unauthenticated write endpoint on the control
plane and needs its own auth decision first.

## Issue #34 — loader silently skips manifests

**Half fixed.** A manifest rejected by `manifest.ParseFile` had come to log a
`Warn` (added with ADR-0002's validation work). Nothing else the walk decided
was visible, and the closing `{"count":0,"message":"scan complete"}` carried no
base directory, no file count, and was logged at `info` — a neutral level for a
state in which no node receives any work.

The specific repro in the issue used `--playbook-dir`, a flag that no longer
exists, so it cannot be reproduced as written. The observability gap it points
at is real and independent of that flag.

Also found: `helvilette.yaml` and `Helvilette.yml` parse fine but are skipped
without a word, because the walk matches `helvilette.yml` exactly. That is the
most likely shape of the original report.

Decision recorded in [ADR-0006](../../../informations/ADRs/ADR-0006.md).

## Defects found in the same pass

Not in any issue; found while reading the code around the three above.

1. **Data race on `Server.loader`.** The sync goroutine wrote it with no lock
   while `GetLoader` read it. `s.mu` existed and guarded `playbooks` but not
   `loader`. Now guarded; `TestEnsureLoader_ConcurrentWithGetLoader` runs under
   `-race`.
2. **Data race on `Loader.playbooks`.** `Scan` rewrote the map from the sync
   goroutine while `Get`/`GetByName` read it from HTTP handlers. Now guarded by
   an `RWMutex` on the loader.
3. **The sync goroutine had no exit.** It outlived graceful shutdown. Now takes
   a `context.Context`, cancelled in `main.go` alongside `SetReady(false)`.
4. **A successful sync was invisible at the default log level.** Logged at
   `debug`. Now `info` when the commit moves, `debug` when it does not.
5. **A half-finished clone never self-healed.** A `destDir` that existed but
   would not open failed on every later poll forever. Now discarded and
   re-cloned — but only on an open failure, never on a fetch failure, so a
   network outage leaves the existing checkout intact.
6. **README documented a removed flag.** `README.md` told operators to pass
   `--playbook-dir`, which `removedFlagError` rejects with exit 1, and did not
   mention any of the fleet flags. Filed as its own issue and fixed.

## Not fixed, for the record

`vagrant/baseline-repo/helvilette.yml` declares `nodeSelector: {}`. Since
ADR-0004, `validateNodeGroup` rejects an empty selector: it matches no node,
which reads as "matches everything" but behaves as "matches nothing". That
manifest is therefore rejected at load time and its playbook is never
dispatched. The directory is gitignored, so this is a note for the next manual
Vagrant run rather than a change: give the group a real selector, e.g.
`nodeSelector: {role: baseline}`, and label the agents to match.
