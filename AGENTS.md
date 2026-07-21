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
