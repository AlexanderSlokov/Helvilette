# Helvilette: AI Agent Backlog & Roadmap

Tài liệu này tổng hợp roadmap và các hạng mục công việc (backlog) nhằm cung cấp lộ trình rõ ràng cho AI Agents (hoặc contributors) khi tham gia phát triển dự án.

## 1. Core Concept & Pivot Review
Trước khi nhận việc, contributor / AI Agent cần nắm vững định hướng sản phẩm:
- **Helvilette là Ansible delivery + drift protection layer**, nằm ở tầng OS/systemd. Nó biến Ansible playbook thành hệ thống tự vận hành mà không cần SSH, không cần CI/CD pipeline, ngăn ngừa config drift.
- Helvilette không cạnh tranh với K8s/Nomad, không phải CI/CD thay thế GitHub Actions.
- Target audience: 12-50 VM, hybrid infra, team 1-2 người.
- Mô hình tương đồng: Othela giống `kube-apiserver` (Control Plane). Helvilette Agent giống `kubelet` (chạy trên các node, kéo việc, thực thi và báo cáo).

### 1.1. Architecture Decisions
- **Hybrid Model cho `helvilette.yml`:** Nằm TRONG Ansible Playbook repo. Chứa metadata + defaults. Secrets/env vars do Othela inject.
- **Logging:** Sử dụng `github.com/rs/zerolog` cho structured logging.
- **Playbook Distribution:** Pull-based. Othela gửi reference, Agent tự clone/pull repo và chạy playbook nội bộ (giống kubelet pull image).
- **Frontend Stack:** Bỏ hướng Wails Desktop & Death Stranding UI phức tạp, chuyển sang Web UI đơn giản, tập trung vào core functionality.

## 2. Current State (Đã hoàn thành)
- BDD E2E Testing Framework (Sử dụng Ginkgo & Gomega).
- Living Skeleton E2E (Agent pull & run Ansible).
- Cấu trúc Server và Agent.
- Ephemeral Testing Environment (Docker Compose với 1 Server + 3 Agents).
- K8s-Style Configuration (Cobra CLI, YAML parser cho Agent config).
- GitOps Playbook Distribution (Agent tự động clone/pull repository và chạy playbook từ Git nội bộ).

---

## 3. High Priority Backlog (Must-Have cho bản Demo / Release tiếp theo)

Các hạng mục này là cốt lõi dựa trên định hướng Pivot, phải xong trước tiên.

### 3.1. Phase 2: GitOps Playbook Distribution (Agent Clone/Pull)
Chuyển đổi từ việc Othela gửi cục `PlaybookContent` sang gửi `Reference` để Agent tự clone từ Git.
- [x] **Job Struct Update:** Cập nhật model `Job` thêm `RepoURL`, `PlaybookPath`, `Version` và loại bỏ `PlaybookContent` dần dần.
- [x] **Agent Git Package (`pkg/git`):**
  - [x] Implement `pkg/git/cache.go` (Local repo cache).
  - [x] Implement `pkg/git/clone.go` (Clone/pull repository bằng `go-git`).
- [x] **Agent Execution Logic Update:** Cập nhật `ExecutePlaybook()`: kiểm tra repo cache -> clone/pull (nếu thiếu hoặc version đổi) -> chạy `ansible-playbook` từ đường dẫn nội bộ.
- [x] **E2E/Integration Tests:** Đảm bảo Othela gửi reference -> Agent pull thành công từ Gitea local và execute.

### 3.2. Node Targeting & Label-Based Routing
Hiện tại Job đang broadcast. Cần gán việc có đích.
- [ ] **Agent side:** Agent gửi thông tin Label/Tag của nó khi gọi API register/sync với Othela.
- [ ] **Othela side:** Định nghĩa `nodeSelector` trong cấu hình (giống Kubernetes). Update dispatcher logic để match `nodeSelector` với Agent's labels trước khi trả về Job.

### 3.3. Persistence Layer cho Othela (SQLite)
Hiện tại Othela lưu trên memory. Cần cơ sở dữ liệu để ghi nhận lịch sử.
- [ ] Tích hợp SQLite driver.
- [ ] Implement tables/models cho **Node Registry** (thông tin các Agent đang kết nối).
- [ ] Implement tables/models cho **Job History** & **Execution Reports**.

### 3.4. Security & Basic Authentication
- [ ] Implement Pre-shared token / API key cho Agent ↔ Othela.
- [ ] Othela endpoint middleware để verify token.
- [ ] Agent cấu hình và gửi header Authorization/Token.

### 3.5. State Awareness & Drift Detection
- [ ] Cho phép Agent định kỳ chạy `ansible-playbook --check` (Drift mode).
- [ ] So sánh state hiện tại với desired state. Nếu có drift, Agent báo cáo `DriftDetected` event về Othela.

### 3.6. Production Readiness
- [ ] Node registration API (để Agent đăng ký node info, capabilities với Othela).
- [ ] Health check endpoints (`/healthz`, `/readyz`).
- [ ] Graceful shutdown handling cho Othela và Agent.
- [ ] Systemd service files (`othela.service`, `helvilette-agent.service`).

---

## 4. Medium Priority Backlog (Nice-to-Have / Post-MVP)

Sau khi hệ thống lõi hoạt động hoàn chỉnh, tiến hành các tính năng bổ sung.

### 4.1. Othela Playbook / Repo Management (Multi-repo support)
- [ ] `pkg/git/repo.go` & `watcher.go`: Othela tự động sync/track các repos.
- [ ] API Endpoints đăng ký, list, và manual sync Repos (vd: `POST /api/v1/repos`).

### 4.2. Webhook Triggers
- [ ] Othela lắng nghe Webhook (từ GitHub/Gitea/GitLab) khi có `git push`.
- [ ] Khi nhận trigger, Othela lập tức notify/invalidate cache các Agents liên quan mà không phải đợi hết Poll Interval.

### 4.3. Health Probes (Systemd Liveness/Readiness)
- [ ] Support `livenessProbe` và `readinessProbe` cho systemd services (K8s style).
- [ ] Agent định kỳ kiểm tra sức khỏe service (HTTP get, TCP socket, Exec) độc lập với vòng lặp của Ansible.

### 4.4. Scheduled Playbook Runs
- [ ] Hỗ trợ Cron-like schedule để tự động trigger job từ phía Othela thay vì chỉ chạy 1 lần.

---

## 5. Low Priority Backlog (Features for V1.x)

### 5.1. Dashboard UI (Web)
*(Lưu ý: Bỏ định hướng Death Stranding UI / Wails Desktop App theo Pivot, chỉ làm Web UI đơn giản).*
- [ ] Danh sách Nodes với trạng thái (Status badges).
- [ ] Trạng thái Job gần nhất.
- [ ] Real-time log streaming qua WebSocket.
- [ ] Playbook catalog browser.

### 5.2. Multi-tenant / Namespace Support
- [ ] Thêm khái niệm `Namespace` để phân chia environment (Dev/Staging/Prod).
- [ ] RBAC giới hạn quyền deploy.

---

## Hướng dẫn Quy trình Làm việc cho AI Agents

Khi nhận task từ Backlog này, AI Agent cần:
1. **Kiểm tra trạng thái:** Đọc file này để xem những phần đã có người làm (File này là Single Source of Truth).
2. **Thiết kế trước khi code (Planning Mode):** Nếu task phức tạp, luôn viết/update file `docs/implementation_plan.md` và đợi approve từ user.
3. **Tuân thủ Logging & Error Handling:** Dùng `zerolog` cho mọi tiến trình. Xử lý lỗi cẩn thận, đặc biệt là khi Agent fail khi chạy playbook (cần report rõ log cho Othela).
4. **Cập nhật Roadmap:** Khi một task trong đây hoàn thành, hãy đánh dấu `[x]` vào chính file này.
