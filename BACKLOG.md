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

- Doctrine and roadmap aligned with ADR-0008: daemon allowed for Helvilette,
  probes section, SSH signing, immutable audit log free, Google SRE 70% figure
  cited instead of the unsourced 80%.
- First #43 fixes on branch `fix/43-agent-state-on-disk`, cut from
  `docs/adr-0008-standalone-agent`. Workspace defaults to
  `/var/lib/helvilette/agent`; state files written atomically
  (`cmd/agent/statefile.go`); `extra_vars` moved out of the checkout;
  `ExecutePlaybook` split.
- e2e suite fixed for the one-node stack (`agentNodes`). `node-2` was scaled
  down on purpose by the owner in `0e2e07c`. `make test` and `make e2e` pass,
  10 of 10 specs.
- ADR-0009 written: manifest `v1alpha2`, one root `helvilette.yml`, ordered
  `spec.playbooks`, `apply` defaults to `plan`, K8s envelope kept, `probes`
  reserved. Branch `docs/adr-0009-manifest-v1alpha2`, cut from
  `fix/43-agent-state-on-disk`.
- Doctrine reworded: it rejects copying Kubernetes' topology mechanically, not
  its file format or its state-handling logic. Absorb selectively.
- Manifest `v1alpha2` implemented in `pkg/manifest` with tests, on branch
  `docs/adr-0009-manifest-v1alpha2`. Nothing calls it yet.
  `ParseRepoManifest` returns `os.IsNotExist` unwrapped for the fallback;
  `SelectPlaybooks` gives playbooks in file order with the first matching
  group. The shared nodeGroup checks now take a field path, so v1alpha2
  errors read `spec.playbooks[i].nodeGroups[j]`.
- All branches pushed. No PR, by the owner's choice.

### Next

Each step is one branch, created with `gh issue develop 43 --base <previous
branch>` so it links to the issue, with tests and a green
`make e2e` before moving on. All of them close items of #43.

1. Agent reads its own repository. `agent.yaml` gains `repoURL` and `branch`
   (ADR-0001 layering, flags and env included). `--othela-url`, `OTHELA_URL`,
   `othelaURL`, `RegisterNode`, `Poll` and `SendReport` are deleted outright.
   Each cycle: `git.EnsureRepo`, then `manifest.ParseRepoManifest` on the root
   `helvilette.yml`, then `manifest.SelectPlaybooks` with the node's labels,
   then run each selection in order, stopping at the first failure
   (ADR-0009 D6). No match: one log line, nothing run (D9).
2. Same branch: e2e follows. Give `nginx-collection` a root `helvilette.yml`
   in `v1alpha2`; point `node-1` at it through `agent.env`; drop the agent's
   `depends_on: othela`; rewrite the dispatch specs in
   `tests/e2e/gitops_test.go` to assert on the agent's own journal and
   `last_run_summary.json`. Othela specs stay as long as the service does.
3. Fallback without `helvilette.yml`: `<fqdn>.yml`, `<hostname>.yml`,
   `local.yml`, plan mode (ADR-0009 D7).
4. Check before apply: `--check --diff` always first; `apply: auto` applies
   only after a passing check; `plan` stops there (ADR-0008 D9,
   ADR-0009 D4). The record says which mode ran.
5. SSH commit signing with at least two allowed signers (ADR-0008 D9).
6. Pinned Ansible and the compatibility matrix (ADR-0008 D6). Needs a short
   design talk with the owner: how it is packaged (venv, tarball, image).

### Waiting on the owner

- The `ansible-pull` scouting (Phase 0) runs on the owner's VMs for at least a
  week. Its result closes the open question in ADR-0008 D2. Do not claim
  advantages over `ansible-pull` beyond the three in D2 until then.

---

## Phase 0: Separate the agent from Othela

Exit: the agent runs one full cycle on its own (pull git, check, apply, write
the record), with Othela stopped.

- [x] ADR-0008 "The agent stands alone; Othela is optional and frozen".
- [ ] Mark Othela as frozen in its package docs and in `cmd/othela/main.go`
      help text. No new features land there until Phase 3.
- [ ] Fix the Law 0 violations in issue #43:
  - [ ] Remove `RegisterNode`, `Poll` and `SendReport` from the agent's run
        path. The agent reads `helvilette.yml` from its own checkout.
  - [ ] Remove `--othela-url`, `OTHELA_URL` and `othelaURL` outright, with no
        deprecation period: nobody uses them yet. No "standalone" flag either;
        standalone is the only mode (ADR-0008 D1). Update the README agent
        config table, `helvilette-agent.service`, and drop the agent node's
        `depends_on: othela` in `e2e.compose.yml`.
  - [x] Default workspace to `/var/lib/helvilette/agent`, not `/tmp`.
  - [ ] Run `--check --diff` before every apply.
  - [x] Write `extra_vars` outside the checkout, atomically, and return the
        error. `last_run_summary.json` is written atomically too.
- [ ] Repo URL, branch and signing keys go in `agent.yaml`, layered as in
      ADR-0001, like `ansible-pull -U -C`. (ADR-0008 D2)
- [ ] Playbook selection: `helvilette.yml` when present, otherwise
      `<fqdn>.yml`, `<hostname>.yml`, `local.yml`. (ADR-0008 D2, was 3.14)
- [x] ADR for the manifest schema after `spec.repo` loses its role:
      ADR-0009, `v1alpha2`. (ADR-0008 D3)
- [x] Implement `v1alpha2` in `pkg/manifest` alongside `v1alpha1`, which
      Othela keeps reading. `ParseRepoManifest`, `DecodeRepoManifest`,
      `ValidateRepoManifest`, `SelectPlaybooks`. `spec.playbooks` list, `apply` defaulting to
      `plan`, paths confined to the repository, `probes` rejected as not yet
      supported, removed fields rejected with a pointer to what replaced them.
      (ADR-0009)
- [ ] Rewrite the e2e fixtures to a root `helvilette.yml` in each playbook
      repository. (ADR-0009)
- [ ] Inventory the tests. Mark which Ginkgo and Testcontainers specs exercise
      the agent alone and which need Othela. Rewrite
      `tests/e2e/gitops_test.go` for the standalone agent.

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
