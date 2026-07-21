# Helvilette: AI Agent Backlog & Roadmap

Tài liệu này tổng hợp roadmap và các hạng mục công việc (backlog) nhằm cung cấp lộ trình rõ ràng cho AI Agents hoặc contributors khi tham gia phát triển dự án.

## Current State (Đã hoàn thành)
- BDD E2E Testing Framework (Ginkgo & Gomega).
- Living Skeleton E2E (Agent pull & run Ansible).
- Cấu trúc Server và Agent.
- Ephemeral Testing Environment (Docker Compose với 1 Server + 3 Agents).
- K8s-Style Configuration (Cobra CLI, YAML parser cho Agent config).
- GitOps Playbook Distribution (Agent tự động clone/pull repository và chạy playbook từ Git nội bộ).
- Node Targeting & Label-Based Routing (Agent registration, label matching, manifest parsing, extra_vars execution).

---

## 3. High Priority Backlog (Must-Have cho bản Demo / Release tiếp theo)

Các hạng mục này là cốt lõi dựa trên định hướng Pivot, phải xong trước tiên.

### 3.1. Phase 2: GitOps Playbook Distribution (Agent Clone/Pull)
Chuyển đổi từ việc Othela gửi PlaybookContent sang gửi Reference để Agent tự clone từ Git.
- [x] Job Struct Update: Cập nhật model Job thêm RepoURL, PlaybookPath, Version và loại bỏ PlaybookContent.
- [x] Agent Git Package (pkg/git): Implement pkg/git/cache.go và pkg/git/clone.go.
- [x] Agent Execution Logic Update: Cập nhật ExecutePlaybook kiểm tra repo cache -> clone/pull -> chạy ansible-playbook từ đường dẫn nội bộ.
- [x] E2E/Integration Tests: Đảm bảo Othela gửi reference -> Agent pull thành công từ Gitea local và execute.

### 3.2. Node Targeting & Label-Based Routing
Phân phối Job dựa trên Node Labels và Registration.
- [x] pkg/manifest package: Parse helvilette.yml thành Go structs.
- [x] Agent labels config: Thêm Labels map[string]string vào AgentConfiguration (CLI --labels, YAML config, ENV AGENT_LABELS).
- [x] Node Registration API: POST /api/v1/nodes/register. Agent gửi nodeID và labels, Othela lưu vào registry.
- [x] Othela dispatcher update: handleSync đọc labels từ registry, match với nodeSelector từ manifest, trả về đúng job và extra_vars.
- [x] Agent ExtraVars execution: Ghi extra_vars ra file JSON và append -e @file vào ansible-playbook command.
- [x] Job struct update: Thêm ExtraVars map[string]string vào pkg/types.Job.
- [x] Unit tests: Parser pkg/manifest và nodeSelector matching.
- [x] E2E update: Agent khớp labels nhận job, agent không khớp nhận 204 No Content.
- [x] Othela Debug Mode: Thêm cờ --log-level=debug, ẩn polling log khỏi mức INFO.

### 3.3. Persistence Layer cho Othela (SQLite)
Hiện tại Othela lưu trên memory. Cần cơ sở dữ liệu để ghi nhận lịch sử.
- [ ] Tích hợp SQLite driver.
- [ ] Implement tables/models cho Node Registry (thông tin các Agent đang kết nối).
- [ ] Implement tables/models cho Job History & Execution Reports.

### 3.4. Security & Basic Authentication
- [ ] Implement Pre-shared token / API key cho Agent ↔ Othela.
- [ ] Othela endpoint middleware để verify token.
- [ ] Agent cấu hình và gửi header Authorization/Token.

### 3.5. State Awareness & Drift Detection
- [ ] Cho phép Agent định kỳ chạy ansible-playbook --check (Drift mode).
- [ ] So sánh state hiện tại với desired state. Nếu có drift, Agent báo cáo DriftDetected event về Othela.

### 3.6. Production Readiness
- [x] Health check endpoints (/healthz, /readyz).
- [x] Graceful shutdown handling cho Othela và Agent.
- [ ] Systemd service files (othela.service, helvilette-agent.service).

---

## 4. Medium Priority Backlog (Nice-to-Have / Post-MVP)

Sau khi hệ thống lõi hoạt động hoàn chỉnh, tiến hành các tính năng bổ sung.

### 4.1. Othela Playbook / Repo Management (Multi-repo support)
- [ ] pkg/git/repo.go & watcher.go: Othela tự động sync/track các repos.
- [ ] API Endpoints đăng ký, list, và manual sync Repos (POST /api/v1/repos).

### 4.2. Webhook Triggers
- [ ] Othela lắng nghe Webhook (từ GitHub/Gitea/GitLab) khi có git push.
- [ ] Khi nhận trigger, Othela lập tức notify/invalidate cache các Agents liên quan mà không phải đợi hết Poll Interval.

### 4.3. Health Probes (Systemd Liveness/Readiness)
- [ ] Mở rộng pkg/manifest/types.go để parse probes section từ helvilette.yml.
- [ ] Support livenessProbe và readinessProbe cho systemd services (K8s style).
- [ ] Agent định kỳ kiểm tra sức khỏe service (HTTP get, TCP socket, Exec) độc lập với vòng lặp của Ansible.

### 4.4. Vault / Secret Integration
- [ ] Mở rộng pkg/manifest/types.go để parse vault section từ helvilette.yml.
- [ ] Support type: exported (đọc secret từ ENV của Othela host).
- [ ] Support type: hashicorp_vault (đọc secret từ HashiCorp Vault API).
- [ ] Agent nhận vault password file path từ Job, inject vào ansible-playbook --vault-password-file.

### 4.5. Scheduled Playbook Runs
- [ ] Hỗ trợ Cron-like schedule để tự động trigger job từ phía Othela thay vì chỉ chạy 1 lần.

---

## 5. Low Priority Backlog (Features for V1.x)

### 5.1. Dashboard UI (Web)
- [ ] Danh sách Nodes với trạng thái (Status badges).
- [ ] Trạng thái Job gần nhất.
- [ ] Real-time log streaming qua WebSocket.
- [ ] Playbook catalog browser.

### 5.2. Multi-tenant / Namespace Support
- [ ] Thêm khái niệm Namespace để phân chia environment (Dev/Staging/Prod).
- [ ] RBAC giới hạn quyền deploy.
