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
- [x] Manifest schema identity: apiVersion `helvilette.io/v1alpha1`, kind `PlaybookDeployment` (ADR-0002, issue #1).
- [x] Manifest validation (issue #13): kiểm tra apiVersion, kind, và các field bắt buộc khi load. Manifest sai bị từ chối kèm thông báo nêu rõ field, giá trị sai và shape mong đợi, thay vì im lặng deploy cho không node nào.
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
- [x] Tích hợp SQLite driver (mattn/go-sqlite3, theo k3s/kine).
- [x] Tách storage interface (pkg/storage): NodeStore, ReportStore.
- [x] Implement in-memory adapter (pkg/storage/memory.go).
- [x] Implement SQLite adapter (pkg/storage/sqlite.go) cho Node Registry va Execution Reports.
- [x] Inject SQLite vào Othela qua ServerConfig, DB path: {data-dir}/server/db/state.db.
- [ ] Implement tables/models cho Job History (ghi lại job đã gửi cho agent nào, khi nào).

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
- [ ] Sau khi probes có types: bật `yaml.Decoder.KnownFields(true)` trong ParseFile để từ chối key lạ (xem ADR-0002, phần Follow-up work). Hiện chưa bật được vì probes và vault xuất hiện trong manifest mẫu mà chưa có struct tương ứng.
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

---

## 6. Technical Debt

Các hạng mục dưới đây phát hiện khi làm ADR-0002 và issue #13, chưa xử lý.
Chưa mở issue riêng cho hạng mục nào, nên phần mô tả ở đây là nguồn context duy nhất.

### 6.1. Node khớp nhiều nodeGroup thì chỉ nodeGroup đầu tiên được chạy

Mức độ: cao. Đây là lỗi đúng nghĩa, không phải nợ kỹ thuật thuần tuý.

`MatchNodeGroups` trả về mọi nodeGroup khớp, nhưng `handleSync` tại `cmd/othela/server.go:302`
lấy `matches[0]` rồi `break`. Các group khớp còn lại bị bỏ im lặng, không log, không báo lỗi.

Manifest e2e đang dính đúng trường hợp này: `standard-proxies` và `high-performance-proxies`
trong `tests/e2e/data/playbooks/nginx-collection/helvilette.yml` có cùng
`nodeSelector: role=edge-proxy`. Node nào mang label đó cũng chỉ nhận `standard-proxies`.
Toàn bộ `high-performance-proxies`, gồm `systemd_memory_limit` và `enable_seccomp_runtime_default`,
là cấu hình chết mà người vận hành không có cách nào biết.

Cần quyết định ngữ nghĩa trước khi sửa code. Ba hướng:
- Từ chối manifest có nhiều nodeGroup khớp chồng nhau, bắt `nodeSelector` phải loại trừ lẫn nhau.
- Gộp `extra_vars` của các group khớp theo thứ tự khai báo, giống cách Ansible gộp vars.
- Giữ nguyên first-match nhưng log WARN nêu tên các group bị bỏ.

- [ ] Chọn ngữ nghĩa cho trường hợp nhiều nodeGroup cùng khớp một node.
- [ ] Bổ sung validation hoặc log tương ứng vào `pkg/manifest` và `handleSync`.
- [ ] Sửa manifest e2e để phản ánh đúng ngữ nghĩa đã chọn.

### 6.2. Fallback HELV_TEST_REPO_URL thành code chết

Mức độ: thấp.

`cmd/othela/server.go:305-310` fallback sang biến môi trường `HELV_TEST_REPO_URL`, rồi sang một
URL hardcode, khi `Manifest.Spec.Repo` rỗng. Từ issue #13, `spec.repo` là field bắt buộc nên
manifest có repo rỗng bị từ chối ngay lúc load. Nhánh fallback không còn tới được với job sinh
từ manifest. Đã ghi trong phần Consequences của ADR-0002.

- [ ] Xoá nhánh fallback và biến `HELV_TEST_REPO_URL` khỏi `docker-compose.e2e.yaml` nếu không
  còn nơi nào dùng.

### 6.3. Manifest trong repo e2e lồng nhau đã lỗi thời so với working tree

Mức độ: trung bình. Đây là bẫy dễ làm người sửa sau hiểu sai.

`tests/e2e/data/playbooks/nginx-collection` là một git repo riêng nằm trong repo chính.
Commit `12b7723` của nó chứa `helvilette.yml` theo định dạng cũ trước cả K8s-style
(`name: nginx-stack`, `defaults:`, không có `apiVersion`). Bản K8s-style đang dùng chỉ tồn tại
ở working tree, chưa từng được commit vào repo lồng đó.

Hệ quả: `git-server` trong `docker-compose.e2e.yaml` phục vụ nội dung đã commit, còn Othela đọc
manifest từ bind mount working tree (`docker-compose.e2e.yaml:24` và `:29`). Hai bên nhìn thấy hai
file khác nhau. Hiện không gây lỗi vì agent chỉ cần `playbook.yml` từ bản clone, nhưng bất kỳ ai
sửa manifest rồi cho rằng git-server phục vụ bản mới đều sẽ hiểu sai.

- [ ] Commit manifest hiện tại vào repo lồng, hoặc ghi rõ trong README của thư mục e2e rằng
  git-server phục vụ bản đã commit còn Othela đọc working tree.

### 6.4. 12 file lệch gofmt

Mức độ: thấp.

`gofmt -l pkg/ cmd/` báo 12 file: `pkg/git/clone.go`, `pkg/playbook/loader_test.go`,
`pkg/playbook/types.go`, `pkg/systemd/client.go`, `pkg/systemd/watcher.go`, `pkg/types/types.go`,
`cmd/agent/main.go`, `cmd/agent/main_test.go`, `cmd/othela/cmd/root.go`, `cmd/othela/main.go`,
`cmd/othela/server.go`, `cmd/othela/server_test.go`.

Tình trạng này có từ trước issue #13. Chạy `make fmt` một lần sẽ dọn sạch, nhưng nên làm trong
một commit riêng để diff không lẫn với thay đổi logic.

- [ ] Chạy `make fmt`, commit riêng.
- [ ] Thêm bước kiểm tra gofmt vào `.github/workflows/ci.yml` để không tái diễn.

### 6.5. make test kéo theo e2e nên luôn đỏ, còn CI thì không chạy e2e

Mức độ: trung bình.

`Makefile:16` định nghĩa `test: go test ./...`, tức là bao gồm cả `tests/e2e`. Suite e2e dùng
testcontainers, tự build `Dockerfile.othela` và `Dockerfile.agent` rồi dựng 4 container. Trên máy
sạch cache, phần dựng này vượt quá timeout mặc định 10 phút của `go test`, nên `make test` kết
thúc bằng `FAIL helvilette/tests/e2e 600s` kể cả khi mọi unit test đều xanh. Đã có target `make e2e`
riêng dùng ginkgo với timeout rộng hơn, nên `test` không cần ôm e2e.

Ở chiều ngược lại, `.github/workflows/ci.yml` chỉ chạy `go vet`, unit test và build. Không có
bước nào chạy e2e. Nghĩa là đường dẫn end-to-end duy nhất, gồm việc Othela nạp `helvilette.yml`
rồi dispatch job, không được kiểm tra tự động ở bất kỳ đâu.

- [ ] Giới hạn `make test` còn `go test ./cmd/... ./pkg/...`, để e2e cho `make e2e`.
- [ ] Thêm job e2e vào CI, hoặc ghi rõ trong README rằng e2e là bước chạy tay trước khi release.
