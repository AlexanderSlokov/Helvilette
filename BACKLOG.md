# Helvilette: Backlog & Roadmap

Roadmap and backlog for maintainers and AI agents working on Helvilette.

## Status: under overhaul

Helvilette is being rebuilt around a standalone agent, per
[ADR-0008](docs/informations/ADRs/ADR-0008.md).

- The agent comes first. It must pull, check, apply and record on its own, with
  no Othela.
- Othela stays in the tree, but its development is frozen until Phase 3.
  Rolling updates and other multi-node deployment strategies will be Othela's
  job.
- Sources: the [design doctrine](docs/references/Software%20Design%20Doctrine%20Follow%20the%20Soviet's%20Weapons.md)
  and the [roadmap](docs/references/Helvilette_revedict.md).
- The previous backlog is archived verbatim in
  [backlog_v0_othela_first.md](docs/references/plans/backlog_v0_othela_first.md).
  A `(was x.y)` tag points to its section number.

Phase 0 is in progress; Phase 1 is the main target. Phase 3 opens only when a
real team needs multi-node coordination.

---

## Next session

Handoff between the owner and the AI agent, so a new session starts here instead
of re-reading the history. Update this section at the end of every session.

### Last session (2026-10-15)

- Doctrine and roadmap aligned with ADR-0008; Doctrine reworded to absorb
  Kubernetes selectively (topology no, file format and state logic yes).
- #43 items 1, 2 and 6 done; ADR-0009 written and `v1alpha2` implemented.
  The agent pulls `repoURL` itself, reads the root `helvilette.yml`, runs
  matching playbooks in order, and never talks to Othela.
- `pkg/git` now clones again when the requested URL differs from the
  checkout's.
- Comments and operator messages rewritten after the owner's review: no
  ADR, issue or BACKLOG references; reasons stated in place. Rule recorded
  in AGENTS.md. Old code still to sweep: see "Open technical debt".
- Verified by hand on the e2e stack, with Othela stopped: first run applied
  nginx; a pushed commit ran about 2 s later; `kill -9` mid-run was detected
  after systemd's restart and the commit ran again to Success.
- Branches stack, each pushed, none merged, no PR by the owner's choice:
  `docs/adr-0008-standalone-agent` → `fix/43-agent-state-on-disk` →
  `docs/adr-0009-manifest-v1alpha2` → `feat/43-agent-reads-own-repo`.
  The last was created with `gh issue develop 43` and is linked to #43.

### Next

Each step is one branch, created with `gh issue develop 43 --base <previous
branch>` so it links to the issue, with tests and a green `make e2e` before
moving on.

1. Ask the owner what the `ansible-pull` scouting found. It may change the
   order below, or answer whether the agent calls `ansible-pull` itself.
2. Check before apply (#43 item 3): `apply: auto` runs `--check --diff`
   first and applies only if it passes; `plan` already stops at the check.
   The record says which mode ran. Today `auto` applies directly, in
   `ansibleRunner.args`, `cmd/agent/executor.go`.
3. Small fixes seen in the manual run:
   - `pkg/git` logs "fleet cache cloned", wording from when only Othela used
     it. Say "repository cloned".
   - A re-run after an interruption overwrites `last_run_summary.json`, so
     the `Failed (Interrupted)` record survives only in the journal. Keep it,
     for example as a field on the next record, until the run record format
     v1 exists.
4. Fallback without `helvilette.yml`: `<fqdn>.yml`, `<hostname>.yml`,
   `local.yml`, plan mode. The hook is the `os.IsNotExist` branch in
   `Agent.selectPlaybooks`, `cmd/agent/cycle.go`.
5. SSH commit signing with at least two allowed signers (#43 item 4). Adds
   `allowedSigners` to `configSettings`.
6. Pinned Ansible and the compatibility matrix (#43 item 5). Needs a short
   design talk with the owner first: venv, tarball or image.
7. Mark Othela as frozen in `cmd/othela` help text and package docs.

### Waiting on the owner

- The `ansible-pull` scouting (Phase 0), started 2026-10-15 on the owner's
  machines. Its result closes the open question about the agent's engine.
  Do not claim advantages over `ansible-pull` beyond probes with
  remediation, on-disk run records, and halt with a reason until then.

---

## Phase 0: Separate the agent from Othela

Exit: the agent runs one full cycle on its own (pull git, check, apply, write
the record), with Othela stopped.

- [x] ADR-0008 "The agent stands alone; Othela is optional and frozen".
- [ ] Mark Othela as frozen in its package docs and in `cmd/othela/main.go`
      help text. No new features land there until Phase 3.
- [ ] Fix the Law 0 violations in issue #43:
  - [x] Remove `RegisterNode`, `Poll` and `SendReport` from the agent's run
        path. The agent reads `helvilette.yml` from its own checkout.
  - [x] Remove `--othela-url`, `OTHELA_URL` and `othelaURL` outright, with no
        deprecation period: nobody uses them yet. No "standalone" flag either;
        standalone is the only mode (ADR-0008 D1). Update the README agent
        config table, `helvilette-agent.service`, and drop the agent node's
        `depends_on: othela` in `e2e.compose.yml`.
  - [x] Default workspace to `/var/lib/helvilette/agent`, not `/tmp`.
  - [ ] Run `--check --diff` before every apply.
  - [x] Write `extra_vars` outside the checkout, atomically, and return the
        error. `last_run_summary.json` is written atomically too.
- [x] Repo URL and branch go in `agent.yaml`, layered as in ADR-0001, like
      `ansible-pull -U -C`. (ADR-0008 D2)
- [ ] Signing keys (`allowedSigners`) in `agent.yaml`. (ADR-0008 D9)
- [x] Playbook selection from the root `helvilette.yml`. (ADR-0008 D2)
- [ ] Fallback without it: `<fqdn>.yml`, `<hostname>.yml`, `local.yml`, plan
      mode. (ADR-0009 D7, was 3.14)
- [x] ADR for the manifest schema after `spec.repo` loses its role:
      ADR-0009, `v1alpha2`. (ADR-0008 D3)
- [x] Implement `v1alpha2` in `pkg/manifest` alongside `v1alpha1`, which
      Othela keeps reading. `ParseRepoManifest`, `DecodeRepoManifest`,
      `ValidateRepoManifest`, `SelectPlaybooks`. `spec.playbooks` list, `apply` defaulting to
      `plan`, paths confined to the repository, `probes` rejected as not yet
      supported, removed fields rejected with a pointer to what replaced them.
      (ADR-0009)
- [x] Rewrite the e2e fixtures to a root `helvilette.yml` in each playbook
      repository. Done for `nginx-collection`; `baseline` has no node since
      `node-2` was scaled down. (ADR-0009)
- [x] Rewrite `tests/e2e/gitops_test.go` for the standalone agent, including
      a spec that runs a commit with Othela stopped. The remaining Othela
      specs are grouped under "Othela, frozen".

### Scout ansible-pull

`ansible-pull` is the nearest rival and the floor: the agent must do at least
what it does.

- [ ] Run `ansible-pull` from a systemd timer on 2-3 VMs, with a real playbook
      repository, for at least one week.
- [ ] Try `--sleep`, `--only-if-changed`, `--check`, `--checkout`,
      `--verify-commit`, and its `<hostname>.yml` / `local.yml` selection.
      Check that `--verify-commit` accepts SSH signatures. (ADR-0008 D9)
- [ ] Break it on purpose: network loss mid-run, power cut, broken playbook,
      two overlapping runs. Record how it reacts and what trace it leaves.
- [ ] Write the comparison table: feature, `ansible-pull` has it or not, how
      Helvilette handles it. Mark each row reuse, wrap or write.
- [ ] Decide whether the agent calls `ansible-pull` as its engine or only
      follows its design. (ADR-0008 D2 open question)

---

## Phase 1: Agent 1.0

An `ansible-pull` upgraded with care: pulls by itself, checks against the
playbook, applies only when allowed, records everything, runs alone.

### Mechanism

- [ ] Long-running daemon under systemd, `Restart=always`. All state that
      matters on disk, written atomically. (ADR-0008 D4, was 3.5)
- [ ] Poll git with a random splay so a fleet does not pull at once. (was 3.6)
- [ ] Three triggers: new commit, periodic resync (`--check --diff`), manual
      operator trigger. (was 3.6, 3.12 `sync now`)
- [ ] Accept only commits signed with an SSH key from the allowed-signers list
      in `agent.yaml`. Reject the rest, keep the last verified commit, record
      the rejected SHA. (ADR-0008 D9)
- [ ] Refuse to start with fewer than two allowed signers (daily key plus
      offline key), with a four-point error message. (ADR-0008 D9)
- [ ] Document key rotation: removing a stolen key from every node through
      SSH or push-mode `ansible-playbook`. (ADR-0008 D9)
- [ ] `helvilette.yml` says per playbook: auto-apply, or plan only.
- [ ] Warn when the share of tasks that cannot run in check mode exceeds a
      threshold. The override is a line in `helvilette.yml`, so it is signed
      too. (was 3.11)
- [ ] Lock against two overlapping runs.
- [ ] Interrupted run: detect, record, re-run from the start. (ADR-0008 D5,
      replaces was 3.10 at-most-once)
- [ ] Every run has a unique ID, written to disk before execution. (was 3.10)
- [ ] A playbook that cuts the network must not lose the run result. The
      record is on disk, so it survives. (was 3.10, 7.1)
- [ ] Ansible fact cache with a TTL shorter than the resync interval. (was 3.6)
- [ ] Secrets from Kallisto through Ansible's built-in Vault lookup. Keep
      `vault-password-file` for `ansible-vault`. (was 4.6)
- [ ] Enable `yaml.Decoder.KnownFields(true)` once the schema covers probes.
      (was 3.7)
- [ ] A repository with `helvilette.yml` still runs with plain
      `ansible-playbook`. (was 3.14)

### Safeguards

- [ ] Marker line on every file the agent manages, following the
      `ansible_managed` convention: managed by Helvilette, which playbook, halt
      before editing by hand.
- [ ] Login banner: this machine's state is held by Helvilette, how many files
      and services, last run, halt state.
- [ ] Record `shell` and `command` tasks as "effect cannot be known", and never
      hide them. (was 3.8 `?`)
- [ ] `helvilette halt --reason` with a reason catalogue: planned maintenance,
      incident firefighting, infrastructure or power incident, experiment,
      other (note required). `helvilette resume` lifts it. (was 3.12 `pause`)
- [ ] A halt never expires. Each cycle writes a syslog warning; the login
      banner shows how long, why and by whom.
- [ ] Destructive-task heuristic (`rm`, `dd`, `mkfs` and similar) flags tasks in
      `plan`. (was 3.11)

### Health probes and remediation

Core feature, opt-in per service in `helvilette.yml` with a reason. (ADR-0008
D7)

- [ ] Parse a `probes` section in `helvilette.yml`. (was 3.7)
- [ ] Liveness probes for systemd services: HTTP GET, TCP socket, exec.
      (was 3.7)
- [ ] Probes run on their own schedule, independent of the Ansible cycle.
      (was 3.7)
- [ ] Remediation playbook runs once, then halts if it fails. Requires opt-in
      per service with a reason, and a passing check run first. (was 3.7)
- [ ] Back off when remediation fails; never loop. (was 3.7
      RestartFailureBackOff)
- [ ] Every remediation run goes into the run record and the journal.

### Commands on the node

Admin commands, all local, all readable with Othela absent. No `apply`: the
rescue tool is `ansible-playbook`.

- [ ] `status`: last run, commit, result, halt state, interrupted run if any.
      Answers known, known but stale, unknown. (was 3.5, 3.10, 3.12, #22)
- [ ] `plan`: check run, with N of M tasks predictable and the list of the
      rest. (was 3.11 preview)
- [ ] `config`: effective configuration with the source of each value, like
      `nginx -T`. Extends `--print-config` from ADR-0001.
- [ ] `explain <playbook>`: why this playbook applies, or not, to this node,
      which commit, who signed it, how to roll back. (was 3.12 `why`)
- [ ] `footprint`: files, units and packages the agent holds, from real runs.
      `shell` and `command` tasks listed as unknown.
- [ ] `halt --reason`, `resume`. See Safeguards.
- [ ] `logs <run>` with three views: summary (default), `--tasks`, `--json`.
      Failing tasks unfold automatically. (was 3.8)
- [ ] `uninstall`: remove the agent, leave managed services running.
      (was 3.12)

### Records and logs

- [ ] Run record format v1 with a version field. Fields only get added. Ships
      with an upgrade path for the record and `agent.yaml`.
- [ ] Ansible callback plugin emitting JSON lines per task. (was 3.8)
- [ ] JSON schema for the record and the task lines. (was 3.8)
- [ ] Translate module results into machine changes: file path, unit name,
      package version. (was 3.8)
- [ ] Human summary in `+ - ~ ?` form. On failure, unfold the failing task and
      a few before it. (was 3.8)
- [ ] One-line run summary to journald per run. Alloy or a similar tool ships
      logs; the agent never does. (was 3.8)
- [ ] No warnings or errors for configuration fields left unset. (was 3.8)
- [ ] Every error states four things: what failed, where, what the agent did
      and did not do, what to do next. Automated tests assert it.
      (was 3.13, 7.1)
- [ ] Every record carries the Ansible version that ran. (ADR-0008 D6)
- [ ] Previous check results are kept for display only. (was 3.6)

### Packaging

- [ ] One Go binary for the agent.
- [ ] Pinned Ansible shipped with it. Operators can bring their own if it meets
      the pinned version. (ADR-0008 D6, issue #43)
- [ ] Publish an Ansible compatibility matrix: the lowest and highest
      major.minor versions Helvilette is tested with. CI runs the suite against
      each end. (ADR-0008 D6)
- [ ] Production systemd unit for the agent. (was 3.5)
- [ ] Install with one line in cloud-init, or bake with Packer.
- [ ] Bash install script, non-interactive via `INSTALL_HELVILETTE_*`, and an
      Ansible bootstrap playbook. (was 4.3)
- [ ] CI test: uninstall leaves no agent and managed services keep running.
      (was 4.3)

### SQLite-style test suite

The test suite is the permanent mud test. Each row is a required behaviour.

| Extreme condition | Required behaviour |
| --- | --- |
| `kill -9` during apply | Next run detects the interrupted run, records it, re-runs from the start |
| Power loss during apply | Same; no state file corrupted |
| Disk full | Stop, say why, never overwrite an old record |
| Git server unreachable | Keep applied state, keep fixing drift against the last verified commit |
| Kallisto unreachable | Skip tasks that need secrets, name each skipped task |
| Commit with a bad signature | Reject, keep the old commit, name the rejected commit |
| Corrupted checkout | Detect, re-clone, record it |
| Clock skew | Record order unaffected; skew recorded |
| New commit arrives mid-run | Finish the current commit; the new one runs next cycle |

- [ ] One test per row.
- [ ] Validate behaviour after systemd restart, stop and reload. (was 3.5)
- [ ] `journalctl -u helvilette-agent` alone is enough to diagnose a run.
      (was 3.5)

### Exit gates

| Test | Pass condition |
| --- | --- |
| Homelab | The author runs the agent on their own infrastructure |
| Mud test | A test fleet is broken at random (`sshd_config`, deleted users, downgraded packages, stopped services); the agent detects and restores every machine, with no error |
| Concorde | 74 minutes of high load with a hot-fix commit pushed midway; no error, no crash, no flapping (was 7.2) |

---

## Phase 2: Public beta

Goal: an operator who does not know the author installs, uses and fixes
Helvilette alone.

- [ ] Reference cards: every key in `helvilette.yml` with syntax, default and
      context, like NGINX directive docs.
- [ ] One-page quickstart: "You have an Ansible repo? Good."
- [ ] Rewrite README Architecture and Bootstrap sections for the standalone
      agent.
- [ ] Recommend Molecule for playbook testing before enabling auto-apply.
      (ADR-0008 D5)
- [ ] Halt and rescue guide with `ansible-playbook`, written for someone
      firefighting at 2 a.m.
- [ ] Tested support matrix: Ubuntu, Debian, RHEL; local VPS, Proxmox, EC2,
      GCP.
- [ ] Testcontainers test for time to first success, from install command to
      first successful apply on a clean VM. (was 3.13)
- [ ] README comparison table including `ansible-pull` and AWX, with maturity
      markers. (was 4.8)

Exit: at least one operator at another company, who does not know the author,
finds Helvilette, installs it, and files an issue from their own environment.

---

## Phase 3: Othela (frozen until a real team needs it)

Invariant: Othela only reads the records agents write, and only writes to git.
The agent has no "Othela mode". Remove Othela and every agent keeps running.

Planned capabilities:

- [ ] Fleet map: which nodes drifted, which are halted, why, since when.
      (was 3.6 DriftDetected, 3.8 fleet summary, 3.12 `fleet`, 5.1)
- [ ] Ordered multi-node work through push-mode `ansible-playbook` with
      `serial`, `delegate_to`, `run_once`. Ansible is the brain; Othela only
      opens a reverse tunnel for nodes behind NAT, and closes it after.
      (was 4.2, 3.12 `tunnel`)
- [ ] Rolling update and gate conditions. (was 3.7 gate)
- [ ] Fleet-wide `freeze` and `unfreeze`. (was 3.12)
- [ ] Persistence: job history, node status (#22), state that survives a
      mid-job restart, ghost and orphan detection. (was 3.3)
- [ ] Enroll token and agent identity lifecycle, revocation of stolen nodes.
      (was 3.4)
- [ ] Protocol versioning, ETag, backoff, fail-safe poll interval. (was 4.1)
- [ ] Multi-repo management and webhook triggers, including the `--fleet-repo`
      webhook, blocked on an authentication decision. (was 4.4, 4.5, 8)
- [ ] Scheduled runs. (was 4.7)
- [ ] Backup and restore of Othela state, with a CI test. (was 3.12, 4.3)

Graduation: k3s-ansible built on a fleet (was 7.1), and a Concorde test for
Othela, including a hot-patch restart under load (was 7.2).

---

## Free and paid boundary

Test: does a lone operator need this to survive an incident? If yes, it is
free. Only what a large organisation needs may be sold.

| Free, forever | May be sold |
| --- | --- |
| The whole agent and every command on the node | RBAC for multiple teams (was 5.2) |
| Halt with reason, rollback to an older commit | SSO, LDAP, custom auth |
| Error messages, run records, logs; immutable audit log | Audit certification of the log (an enterprise need, deferred) |
| Basic trail: who changed what, when, halt reasons | Compliance packs: FIPS 140, SOC 2, ISO 27001 |
| Basic fleet map in Othela | Multi-site, multi-cluster coordination; namespaces (was 5.2) |

- [ ] Keep commercial code in its own repository from the start. (was 5.3)
- [ ] Publish the boundary commitment: API and data export are never behind a
      paywall. (was 5.3)
- [ ] Set up a CLA before merging the first external PR. (was 5.4)
- [ ] License stays Apache-2.0.

---

## Not doing

- No `helvilette apply` or `apply --force`. The rescue tool is
  `ansible-playbook`. (was 3.12)
- No `.previewed` file and no TTL lease between preview and apply. Without an
  `apply` command there is nothing to bind. (was 3.11)
- No agent behaviour that depends on Othela being up. Resolved by ADR-0008 D1.
  (was 3.9)
- No custom vault types in `helvilette.yml` (`exported`, `hashicorp_vault`).
  Ansible's own lookups do this. (was 4.6)
- No cluster build or rebuild. That is Terraform and cloud-init.
- No replacement for Ansible.
- No polished UI. Plain is allowed; making the operator suffer is not.
- Never lift a halt automatically; never reverse an operator's decision.

---

## Risks

- Ansible and Python on every machine are the heaviest part. Unpinned, every
  machine runs differently. (ADR-0008 D6)
- Check mode has blind spots: `shell` and `command` tasks are often skipped.
  Reports must say so, for example "10 tasks checked, 8 confirmed, 2 unknown:
  shell commands".
- The commit signing key is the only lock. Answered by ADR-0008 D9: SSH keys,
  at least two, trust list on the node. The remaining risk is a stolen key:
  removing it needs a channel outside the repository.
- Syslog warnings are useless unless something ships them to a person.
- On cloud, Helvilette competes with the landlord's tools (AWS Systems Manager,
  GCP configuration management). Its only edge is one method everywhere.

---

## Open technical debt

Carried from the archived backlog. Resolved items stay there.

- [ ] Rewrite code comments and operator-facing messages that point to ADRs,
      issues or BACKLOG sections, following the comment rules in AGENTS.md.
      The agent, `pkg/manifest` v1alpha2, `pkg/git` and the e2e suite are
      done. Left: `cmd/othela`, `pkg/log`, `pkg/playbook`, `pkg/storage`,
      `pkg/manifest` v1alpha1 tests, the `Makefile`, and the deployment unit
      files. Find them with `grep -rn "ADR-[0-9]\|issue #" --include=*.go .`.

- [ ] e2e in CI, or document it as a manual pre-release step. Issue #24.
      Needs image layer caching and `timeout-minutes`. (was 6.5)
- [x] `Agent.ExecutePlaybook` in `cmd/agent/executor.go` was about 110 lines.
      Split into one function per step, each within 20 lines. (was 8, #43)
- [ ] `execStartOf`, `longFlagsIn` and `syslogIdentifierOf` are duplicated
      between `cmd/othela/deployment_test.go` and
      `cmd/agent/deployment_test.go`. Revisit if a fourth guard appears.
      (was 8)
- [ ] Git credentials for private repositories: token, credential file or SSH
      key, and where it lives on the node. Now an agent-only question.
      (was 8)
- [ ] `git clean` in the checkout. `EnsureRepo` hard-resets but leaves
      untracked files, so a stale `helvilette.yml` could still load. (was 8,
      ADR-0005)
- [ ] Subset-overlap rejection for `nodeSelector`, deferred to the next schema
      by ADR-0004. (was 8)
- [ ] e2e asserts that a playbook ran, not that the right playbook ran on the
      right node. Assert the run ID. (was 8)
- [ ] `make test` does not run `-race`; CI does. (was 8)
