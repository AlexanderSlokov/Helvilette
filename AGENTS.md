# Helvilette - Declarative Continuous Delivery for Ansible Playbooks.

## 0. Workflow process

### When pick up task from Backlog

1. Check status: Read this file to see what has been done/is being done.
2. Plan before coding (Planning Mode): If task is complex, always write/update file docs/implementation_plan.md and wait for approval.

### During coding

1. Logging & Error Handling: Use zerolog for all processes. Handle errors explicitly, especially the behavior of Agent when operating, running playbooks, communicating with Apiserver and executing actions on its behalf.
2. Update Roadmap: When a task in this file is completed, mark it as [x] in the corresponding line in this file.

### When edit BACKLOG file

1. Write sentences short, concise, clear, and accurate.
2. Ensure enough context and meaning to be understood by both humans and other AI Agents.
3. Do not use emojis.
4. Do not overuse bold, italic to highlight, except for keywords, concepts, file names, important names.

## 1. Core Concept

- Helvilette is a Ansible delivery + drift protection layer (Declarative GitOps Continuous Delivery for Ansible), running at OS/systemd layer. It turns Ansible playbook into a self-operating system (desired-state reconciliation loop) without SSH.

- In Scope:
  - GitOps Pull-based model, eliminate inbound SSH access and config drift.
  - Agent architecture optimized for hybrid infra, Edge computing and IoT devices.
  - Target audience: 12-50 VM, team 1-2 people.

- Out of Scope:
  - Container orchestration.
  - General-purpose CI/CD pipelines.
  - Infrastructure provisioning.
  - Core configuration management.

- Similar model: Othela is like kube-apiserver (Control Plane). Helvilette Agent is like kubelet (runs on nodes, pulls jobs, executes and reports back).

### 1.1. Architecture Decisions
- Hybrid Model for helvilette.yml: Located in Ansible Playbook repo. Contains metadata and defaults. Secrets/env vars injected by Othela.
- Logging: Use github.com/rs/zerolog for structured logging.
- Playbook Distribution: Pull-based. Othela sends reference, Agent clones/pulls repo and runs playbook internally.
- Frontend Stack: Change to simple Web UI, focused on core functionality.

## 2. Code Guide

### Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid explicit names like `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Types: explicit. No `any`, no `Dict`, no untyped functions.
- No code duplication. Extract shared logic into a function/module.
- Early returns over nested ifs. Max 2 levels of indentation.
- Exception messages must include the offending value and expected shape.

### Comments

- Keep your own comments. Don't strip them on refactor — they carry
  intent and provenance.
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Docstrings on public functions: intent + one usage example.
- Reference issue numbers / commit SHAs when a line exists because
  of a specific bug or upstream constraint.

### Tests

- Tests run with a single command: `<project-specific>`.
- Every new function gets a test. Bug fixes get a regression test.
- Mock external I/O (API, DB, filesystem) with named fake classes,
  not inline stubs.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.

### Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

### Structure

- Follow the framework's convention (Rails, Django, Next.js, etc.).
- Prefer small focused modules over god files.
- Predictable paths: controller/model/view, src/lib/test, etc.

### Formatting

- Use the language default formatter (`cargo fmt`, `gofmt`, `prettier`,
  `black`, `rubocop -A`). Don't discuss style beyond that.

### Logging

- Structured JSON when logging for debugging / observability.
- Plain text only for user-facing CLI output.