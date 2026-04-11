# Helvilette Development Journal

## Session: 2026-04-11 — Ephemeral Cluster & K8s-Style Configuration

### Sự kiện đáng chú ý
**Dựng thành công môi trường giả lập (Ephemeral Lab) và chuẩn hóa cấu hình hệ thống.**

Thay vì test trên một máy WSL2 với các tham số hard-code, hệ thống giờ đây chạy như một tiểu cụm (mini-cluster) bao gồm 1 Control Plane và 3 Worker Nodes qua Docker Compose.

---

### Tasks hoàn thành trong session

#### 1. Môi trường Ephemeral Lab (Phase 1.5)
- Xây dựng thành công `Dockerfile.othela` siêu nhẹ dựa trên Debian slim.
- Xây dựng thành công `Dockerfile.agent` dực trên Ubuntu 24.04 tích hợp sẵn Ansible.
- Tạo một cụm Docker Compose chuẩn: 1 Othela `control-plane` và 3 `nodes` kết nối nhau qua internal network.
- Đồng bộ Go version lên 1.25.6.
- Tổ chức lại Workflow bằng Makefile (`make up`, `make down`, `make logs`).

#### 2. K8s-Style Configuration (Phase 1.6)
- **Othela:** Chuyển đổi Othela từ hard-code sang sử dụng CLI Flags qua `cobra` (theo phong cách `kube-apiserver`). Hệ thống hỗ trợ `--port`, `--data-dir`, `--log-level`.
- **Agent:** Chuyển đổi Agent sử dụng file YAML kết hợp Flags (theo phong cách `kubelet`).
  - Định nghĩa struct `AgentConfiguration` chuẩn xác.
  - Tích hợp `gopkg.in/yaml.v3` và parser theo thứ tự ưu tiên: CLI Flags > YAML Config > ENV > Defaults.
  - Xóa bỏ việc bị hardcoded `NODE_ID=agent-01`, Agent nay nhận ID từ `/var/lib/helvilette/agent.yaml`.

---

### Phân tích giá trị của milestone này

#### 1. Tại sao lại cần môi trường Ephemeral?
Với các hệ thống phân tán, việc không có công cụ spin-up nhanh các Node sẽ khiến quá trình kiểm thử gặp nút thắt (bottleneck). Giờ đây, chỉ bằng 1 lệnh `make up`, tôi có ngay một cụm với 3 Agent gửi poll về Othela liên tục, giúp quan sát rõ được luồng log và test được hiệu suất network nội bộ.

#### 2. Phát hiện Critical Bug trong kiến trúc "Content-Based"
Sau khi chạy thử cụm Docker Compose, một lỗi chí mạng đã xuất hiện trong luồng Playbook Execution:
```json
{"level":"warn","component":"executor","error":"chdir /app/helvillette/othela/data/playbooks/nginx-collection: no such file or directory","job_id":"job-e3edf2135582d6a3","message":"playbook execution failed"}
```
**Nguyên nhân gốc rễ (Root Cause):** Othela truyền PlaybookPath cứng (Absolute path trên Server) qua cho Agent. Nhưng Agent lại chạy trong một Container hoàn toàn tách biệt (không hề có thư mục đó). 
=> Lỗi này chính là chất xúc tác cực kỳ quan trọng chứng minh rằng: **Phải thực thi Phase 2: GitOps Playbook Distribution ngay lập tức!** Agent cần tự clone playbook về workspace nội bộ của nó.

---

### Lessons Learned

1. **Kubernetes Design Patterns là kim chỉ nam tốt:** Việc cấu hình Othela như `apiserver` (chỉ dùng cờ) và Agent như `kubelet` (dùng file yaml `config.yaml` kết hợp cờ `--config`) giúp phân tách vai trò cực kỳ rõ ràng, dễ bảo trì.
2. **Path Dependency Trap:** Tuyệt đối không bao giờ được gửi Absolute Path từ một máy tính này sang một máy tính khác trong các mô hình Client-Server. Mọi thứ phải được giải quyết thông qua Relative Path và Git Repos.
3. **Go Module Syncing in Docker:** Nhớ chạy `go mod tidy` trong quá trình build Docker (Multi-stage) để tránh việc lệch `go.sum` khi thêm mới các module (như `cobra` và `yaml.v3`).

---

### Next Steps (Prioritized)

1. [ ] **Phase 2.1: GitOps Job Struct** - Cập nhật struct `Job` trong `pkg/types/types.go` sang mô hình Reference-based (Git URL, Version) thay vì Content-based.
2. [ ] **Phase 2.2: Git Clone Logic** - Viết module cho Agent để có khả năng tự `git clone` hoặc pull repository tại WorkspaceDir của nó trước khi thực thi lệnh Ansible.
3. [ ] **Pull Request & Code Review** - Commit toàn bộ những thay đổi về môi trường Lab lên nhánh chính trước khi bước sang tính năng GitOps.

---

*Session end: 2026-04-11 14:40 UTC*
*Total time: ~1.5 hours*
*Cụm test: 1 Othela, 3 Agents (Dockerized)* 🚀

## Session: 2026-02-01 — The Living Skeleton Comes Alive

### Sự kiện lịch sử
**Lần đầu tiên Helvilette thực hiện end-to-end deployment thực sự.**

Lúc `10:10:28 UTC`, Agent nhận job từ Othela, chạy Ansible playbook, và cài đặt NGINX thành công lên WSL2 Ubuntu.

```
2026-02-01T10:10:11Z INF processing new job has_path=true job_id=job-e3edf2135582d6a3
2026-02-01T10:10:28Z INF playbook execution succeeded
2026-02-01T10:10:28Z INF sending report to Othela status=Success

● nginx.service - Active: active (running) ✅
```

---

### Tasks hoàn thành trong session

#### 1. Tách Shared Types (`pkg/types`)
- Tạo `pkg/types/types.go` với `Job` và `Report` structs
- Cập nhật `cmd/othela` và `cmd/agent` để sử dụng shared types
- Foundation cho việc mở rộng Dashboard/UI sau này

#### 2. Playbook Loader (`pkg/playbook`)
- `types.go` - Playbook metadata struct với ID generation
- `loader.go` - Scanner phát hiện playbooks từ disk
- `loader_test.go` - 8 unit tests (87.8% coverage)
- Methods: `Scan()`, `Load()`, `Get()`, `GetByName()`

#### 3. Othela Server Updates
- `NewServerWithLoader()` - Constructor với playbook loader
- `GET /api/v1/playbooks` - API endpoint list playbooks
- Job bao gồm `PlaybookPath` để enable role resolution

#### 4. Agent Enhancements
- **PlaybookPath support:** Chạy từ đúng thư mục collection
- **Zerolog integration:** Structured logging toàn bộ execution flow
- `cmd.Dir = filepath.Dir(job.PlaybookPath)` - Key change cho role resolution

#### 5. Sample nginx-collection
```
nginx-collection/
├── helvilette.yml      # Collection metadata
├── playbook.yml        # Entry point (become: yes, connection: local)
└── roles/install_nginx/tasks/main.yml
```

---

### Phân tích giá trị của milestone này

#### Câu hỏi: "Chưa ship collection repo về VM, nhưng sự kiện này có đáng giá không?"

**Trả lời: CÓ. Rất đáng giá.**

##### 1. Proof of Architecture ✅
Kiến trúc Pull-based đã được chứng minh hoạt động:
- Agent → Poll → Othela
- Othela → Serve playbook từ disk
- Agent → Execute với role resolution
- Agent → Report kết quả

##### 2. Critical Path Validated ✅
Các thành phần quan trọng nhất đã hoạt động:
- Go wrapper điều khiển Ansible thành công
- JSON callback capture output chính xác
- Playbook với roles phức tạp chạy được

##### 3. Technical Risks Eliminated ✅
Những rủi ro lớn đã được loại bỏ:
- ✓ `ANSIBLE_STDOUT_CALLBACK=json` hoạt động ổn định
- ✓ Role resolution với `cmd.Dir` approach hoạt động
- ✓ Structured types có thể serialize/deserialize qua HTTP

##### 4. Foundation cho phần còn lại ✅
Những gì cần làm tiếp đã rõ ràng và CÓ THỂ LÀM ĐƯỢC:
- Git Watcher: Clone/pull repo về `data/playbooks/` → Loader sẽ discover
- Bundle shipping: Zip playbook + roles → Agent download → giải nén
- Remote Agent: Chỉ cần để Agent chạy trên VM khác, Othela expose API

##### 5. Psychological Milestone 🎯
**Living Skeleton actually lived.**

Không còn là theory hay mockup. Hệ thống đã cài đặt phần mềm thực (NGINX) lên máy thực (WSL2) thông qua flow hoàn chỉnh.

---

### Lessons Learned

1. **PlaybookPath là critical** - Không có nó, roles không resolve được
2. **Structured logging sớm = debug dễ** - Zerolog giúp trace flow rõ ràng  
3. **Test coverage pays off** - 87.8% coverage cho loader giúp tự tin refactor

---

### Next Steps (Prioritized)

1. [ ] Git Watcher - Clone playbook repos về `data/playbooks/`
2. [ ] Job Selection API - Cho phép chọn playbook cụ thể
3. [ ] Bundle Shipping - Zip và gửi playbook tới remote Agent
4. [ ] Node Registration - Agent đăng ký với Othela khi khởi động

---

*Session end: 2026-02-01 10:17 UTC*
*Total time: ~1 hour*
*Lines of code added: ~500*
*NGINX installed: 1* 🎉