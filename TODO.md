# Helvilette Future Roadmap & TODOs

## ✅ Completed Milestones

### Session 2026-02-01: Living Skeleton E2E
- [x] **Shared Types** - `pkg/types/types.go` với `Job`, `Report`
- [x] **Playbook Loader** - `pkg/playbook/` (Scan, Load, Get) với 87.8% coverage
- [x] **Othela Integration** - `NewServerWithLoader()`, `GET /api/v1/playbooks`
- [x] **Agent PlaybookPath** - Chạy từ đúng thư mục, roles resolve
- [x] **Agent Zerolog** - Structured logging toàn bộ execution flow
- [x] **nginx-collection** - Sample collection cho testing
- [x] **First Real Deployment** - NGINX installed on WSL2 via full E2E flow!

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

## 1. Phase 1.5: Ephemeral Testing Environment (Docker Compose) 🚧 NEW
**Mục tiêu:** Tạo một môi trường lab cục bộ dùng Docker Compose để giả lập một cụm bao gồm 1 Othela (Control Plane) và nhiều Agent (Nodes). Cụm này có thể dễ dàng spin up / tear down để kiểm thử các tính năng phân phối Job.

### 1.5.1. Dockerfiles
- [ ] Xây dựng `Dockerfile.othela`: Build binary cho Othela server, sử dụng base image nhỏ gọn gọn nhẹ (Debian slim LTS)
- [ ] Xây dựng `Dockerfile.agent`: Build binary cho Agent. **Lưu ý quan trọng:** Base image cho agent cần cài đặt sẵn `ansible` và các dependencies cần thiết để chạy playbooks (Ubuntu 24.04)

### 1.5.2. Docker Compose Configuration (`docker-compose.yaml`)
- [ ] Cấu hình Othela service (port mapping 8080:8080, volumes cho dữ liệu trạng thái nếu cần)
- [ ] Cấu hình 3+ Agent services (`node-1`, `node-2`, `node-3`)
- [ ] Setup network cho phép các Agent kết nối tới Othela thông qua hostname nội bộ (e.g., `http://othela:8080`)
- [ ] Mount volumes (tuỳ chọn) cho Agents để dễ dàng debug playbook execution hoặc cache.

### 1.5.3. Development Workflow (Makefile)
- [ ] Thêm lệnh `make up` (build images + `docker-compose up -d`)
- [ ] Thêm lệnh `make down` (`docker-compose down -v`)
- [ ] Thêm lệnh `make logs` (xem logs tổng hợp)
- [ ] Cập nhật tài liệu README.md cách dùng môi trường dev

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
- [ ] Configuration via env/config file
- [ ] Systemd service files
