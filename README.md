# Helvilette: The Walking Skeleton (PoC)

Proof of Concept for **Helvilette** – An OS Service Orchestration Framework.
This project demonstrates the "Outbound-only" agent architecture where a Go binary acts as a wrapper around Ansible, 
pulling jobs from a central Control Plane (Othela).

## 1. Prerequisites

- **Docker** and **Docker Compose** (for testing environment)
- **WSL 2 (Ubuntu 24.04)** or any Linux environment (for manual testing)
- **Go 1.25.6** (Verified with your installation at `/usr/local/go/bin/go`).
- **Ansible** (`sudo apt install ansible`).

## 2. Directory Structure

```text
helvilette/
├── cmd/
│   ├── othela/      # Control Plane (Server)
│   └── agent/       # Node Agent (Client)
├── pkg/             # Shared packages
├── data/configs/    # K8s-style configuration files for nodes
├── helvillette/     # Playbook configurations
└── docker-compose.yaml
```

## 3. How to Run 

### Method A: Ephemeral Testing Environment (Recommended)

You can spin up a complete testing cluster containing one Control Plane (Othela) and three Node Agents using Docker Compose. The environment is configured with Kubernetes-style CLI flags and YAML configurations.

```bash
# Build images and start the cluster
make up

# View real-time logs from all nodes
make logs

# Tear down the cluster and remove volumes
make down
```

### Method B: Manual Sanity Test (WSL/Local)

We will open two terminals in WSL to simulate the Server and the Client.

#### Terminal 1: Othela (Control Plane)
Start the server listening on port 8080.
```bash
cd /mnt/e/Helvilette
/usr/local/go/bin/go run ./cmd/othela --port=8080 --data-dir=helvillette/othela/data/playbooks --log-level=info
```

#### Terminal 2: Helvilette Agent
Start the agent. It will poll Othela every 5 seconds.
You can use environment variables, CLI flags, or a config file (Kubelet style).
```bash
cd /mnt/e/Helvilette
# Using CLI flags
/usr/local/go/bin/go run ./cmd/agent --othela-url=http://localhost:8080/api/v1 --node-id=agent-local --poll-interval=5s
```

### Expected Behavior
1. **Agent** connects to Othela.
2. **Othela** sends a "Job" containing an Ansible Playbook (Prints "Hello Wunjo!").
3. **Agent** saves this to its configured workspace directory.
4. **Agent** runs `ansible-playbook -i "localhost," -c local` with `ANSIBLE_STDOUT_CALLBACK=json`.
5. **Agent** captures the JSON output and sends it back to Othela.
6. **Othela** prints the JSON report to the console.

## 4. Architectural Sanity Check (The "Wet Dream" or Reality?)

### Q1: Is `os/exec` with `ANSIBLE_STDOUT_CALLBACK=json` reliable?
**Answer:** **Yes, but with caveats.**
- **Pros:** It turns unstructured text logs into structured data without complex regex parsing. Modern Ansible versions support this natively and robustly. It effectively turns Ansible into an API.
- **Cons/Risks:** 
    - **Streaming encoding:** Ansible buffers output. If a playbook is huge, you won't see "real-time" logs until a task finishes. For long-running tasks, the feedback loop might feel slow.
    - **Crash Handling:** If the Ansible python process SEGFAULTs or is killed by OOM, it won't produce JSON. The Go wrapper must handle `signal: killed` or non-JSON stderr gracefuly (implemented in `cmd/agent/main.go`).

### Q2: Risks of Pull-Based Model (Poll vs. Push)?
**Answer:** The Pull model is the correct choice for this scale.
- **Availability Risk:** If Othela is down, Agents keep running their *current state* (which is good). However, they cannot report health or receive updates.
- **Thundering Herd:** If 10,000 agents wake up and poll at the exact same millisecond, Othela will die. **Mitigation:** In production, you MUST implement "Jitter" (randomize the 5 s sleep to 5 s ± 2 s) to spread the load.

## 5. Configuration (Kubernetes Style)

Helvilette adopts Kubernetes-style configuration patterns:
- **Othela (Control Plane):** Acts like `kube-apiserver`, receiving configuration entirely through CLI flags (`--port`, `--data-dir`, `--log-level`).
- **Agent (Node):** Acts like `kubelet`, using a combination of a YAML configuration file (`--config=agent.yaml`) and CLI overrides.

### Agent Configuration Priority
1. CLI Flags (Highest)
2. YAML Configuration File (`--config`)
3. Environment Variables (`OTHELA_URL`, `NODE_ID`, `POLL_INTERVAL`)
4. Default Values (Lowest)

## 6. Summary
The architecture is **FEASIBLE**. The Ephemeral Cluster proves that Go can drive Ansible, manage Kubernetes-style configurations, and close the loop with the Control Plane inside isolated containers.