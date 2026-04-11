# Helvilette Future Roadmap & TODOs

## ✅ Completed Milestones

### Session 2026-02-01: Living Skeleton E2E
- [x] **Shared Types** - `pkg/types/types.go` với `Job`, `Report`
- [x] **Playbook Loader** - `pkg/playbook/` (Scan, Load, Get) với 87.8% coverage
- [x] **Othela Integration** - `NewServerWithLoader()`, `GET /api/v1/playbooks`
- [x] **Agent PlaybookPath** - Chạy từ đúng thư mục, roles resolve
- [x] **Agent Zerolog** - Structured logging toàn bộ execution flow
- [x] **nginx-collection** - Sample collection for testing
- [x] **First Real Deployment** - NGINX installed on WSL2 via full E2E flow!

### Phase 1.5: Ephemeral Testing Environment (Docker Compose) ✅
- [x] Xây dựng `Dockerfile.othela` (Go 1.25.6, Debian slim LTS)
- [x] Xây dựng `Dockerfile.agent` (Go 1.25.6, Ubuntu 24.04 + Ansible)
- [x] Cấu hình `docker-compose.yaml` với 1 Othela + 3 Agents
- [x] Cập nhật `Makefile` với các lệnh quản lý cluster (`make up`, `make down`, `make logs`)
- [x] Xử lý truyền `NODE_ID` và cấu hình qua ENV

---

## 0. Architecture Decisions (Đã chốt)

### 0.1. Hybrid Model cho `helvilette.yml`
**Quyết định:** `helvilette.yml` nằm **TRONG** Ansible Playbook repo (như `Chart.yaml` của Helm)

**Lý do:**
- `helvilette.yml` chứa **metadata + defaults** (version, description, profiles, default vars)
- **Secrets + environment-specific values** được Othela inject tại runtime (không commit vào Git)
- GitOps-friendly: Một commit = một phiên bản hoàn chỉnh
- Secrets management: Othela giữ secrets trong SQLite/etcd, hoặc tích hợp Vault/SOPS

**Cấu trúc mẫu:**
```
playbooks-repo/
├── nginx-stack/
│   ├── helvilette.yml      # Metadata + defaults
│   ├── requirements.yml    # Ansible Galaxy deps
│   ├── playbook.yml
│   └── roles/
```

### 0.2. Logging Library: `zerolog`
**Quyết định:** Sử dụng `github.com/rs/zerolog` cho structured logging ✅ **Implemented**

### 0.3. Frontend Stack
**Quyết định:** Wails v2 + Vue/Svelte + TailwindCSS + DaisyUI
**Design:** Death Stranding-inspired (Industrial Brutalism + High-Tech Minimalist)

### 0.4. Playbook Distribution: K8s-style (2026-02-01)
**Quyết định:** Agent tự clone/pull playbook repos (như Kubelet pull images)

**Pattern:**
```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│     Othela      │ ──────► │      Agent      │ ──────► │   Git Repo      │
│                 │ JobSpec │                 │  Clone/ │                 │
│ (Control Plane) │ (JSON)  │   (Node Agent)  │  Pull   │                 │
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

**Lý do:**
- Bandwidth hiệu quả (chỉ gửi reference, không gửi payload)
- Agent cache repos locally
- Version control via Git SHA
- Giống cách K8s kubelet hoạt động

---

## 1. Phase 1.6: Kubernetes-Style Configuration 🚧 NEW
**Mục tiêu:** Quản lý cấu hình linh hoạt và chuẩn chỉ theo mô hình của Kubernetes: Othela (Control Plane) dùng Command-line Flags (giống `kube-apiserver`), còn Agent (Node) dùng YAML file kết hợp Flags (giống `kubelet`).

### 1.6.1. Othela CLI Flags (`kube-apiserver` style)
- [ ] Tích hợp package CLI (`cobra`)
- [ ] Hỗ trợ các flags cơ bản: `--port`, `--data-dir`, `--log-level`
- [ ] Bỏ hardcode port 8080 trong `cmd/othela/main.go`

### 1.6.2. Agent YAML Config (`kubelet` style)
- [ ] Tích hợp yaml parser (e.g., `gopkg.in/yaml.v3`)
- [ ] Định nghĩa struct `AgentConfiguration` tương đương (chứa `OthelaURL`, `NodeID`, `PollInterval`, `WorkspaceDir`)
- [ ] Cho phép Agent nhận flag `--config=/path/to/config.yaml`
- [ ] file cấu hình được mount vào container tại `/var/lib/helvilette/agent.yaml`
- [ ] **Thứ tự ưu tiên cấu hình:** CLI Flags > YAML Config > Environment Variables > Defaults

### 1.6.3. Docker Compose Update
- [ ] Tạo các file YAML config mẫu cho 3 Agents (`agent-01.yaml`, `agent-02.yaml`, `agent-03.yaml`) trong thư mục `data/configs/`
- [ ] Mount thư mục configs vào Docker container
- [ ] Sửa command chạy Agent trong `docker-compose.yaml` để truyền `--config` flag thay vì chỉ dùng ENV.

---

## 2. Phase 2: GitOps Playbook Distribution 🎯 NEXT

### 2.1. Job Struct Update
```go
type Job struct {
    JobID        string `json:"job_id"`
    
    // K8s-style: Reference-based
    RepoURL      string `json:"repo_url,omitempty"`      // git@github.com:org/playbooks.git
    PlaybookPath string `json:"playbook_path,omitempty"` // nginx-collection/playbook.yml  
    Version      string `json:"version,omitempty"`       // commit SHA hoặc tag
    
    // Legacy: Content-based (fallback)
    PlaybookContent string `json:"playbook_content,omitempty"`
}
```

### 2.2. Othela Components
- [ ] `pkg/git/repo.go` - Git repository abstraction
- [ ] `pkg/git/watcher.go` - Periodic sync từ remote repos
- [ ] `pkg/git/registry.go` - Manage multiple repos
- [ ] API: `POST /api/v1/repos` - Register new repo
- [ ] API: `GET /api/v1/repos` - List registered repos
- [ ] API: `POST /api/v1/repos/{id}/sync` - Trigger manual sync

### 2.3. Agent Components  
- [ ] `pkg/git/cache.go` - Local repo cache management
- [ ] `pkg/git/clone.go` - Clone/pull operations
- [ ] Update `ExecutePlaybook()` to:
  1. Check if repo exists locally
  2. Clone if missing, pull if version changed
  3. Execute from cached path

### 2.4. Verification
- [ ] Unit tests cho git package
- [ ] Integration test: Othela register repo → Agent clone → Execute
- [ ] Test với real GitHub repo

---

## 3. Agent Intelligence (State Awareness)
- [ ] **Drift Detection:** `ansible-playbook --check` trước khi apply
- [ ] **Security:** mTLS cho kết nối giữa Agent và Othela

---

## 4. UI/Dashboard
### 4.1. Core Features:
- [ ] Danh sách Node với status badges
- [ ] Trạng thái Job gần nhất
- [ ] Log realtime (Stream qua WebSocket)
- [ ] Playbook catalog browser

### 4.2. Design Reference:
- **Style:** Death Stranding Terminal UI
- **Fonts:** SST Roman, Sackers Gothic, Monospace cho data
- **Colors:** Dark base + neon accents (cyan, orange)
- **Effects:** Subtle glitch, hologram glow (không quá nặng)

---

## 5. Production Readiness
- [ ] Node registration với Othela
- [ ] Health check endpoints
- [ ] Graceful shutdown handling
- [ ] Systemd service files