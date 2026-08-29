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
- [ ] Thiết kế schema lưu trạng thái lần chạy trước trên Othela. Món 3.8 (Structured Logging) sẽ ép trả lời câu này, nên tốt nhất quyết định trước.
- [ ] **Trạng thái job phải nằm trong SQLite, không trong RAM Othela.** Othela restart giữa chừng mà job nằm trong RAM = mất hết. Session Concorde test chốt: nếu Othela giữ "ai đang chạy gì" trong RAM, nó trượt bài hot-patch.
- [ ] **Ghost/orphan detection.** Agent tự đăng ký, nên Othela có khả năng phát hiện: node có trong inventory nhưng không tồn tại (ghost), node đang chạy nhưng không có trong inventory (orphan). Ansible mù hoàn toàn ở chỗ này -- inventory mục nát ngay từ tuần đầu.

### 3.4. Enroll Token & Agent Identity Lifecycle

Agent đăng ký với Othela bằng one-time enroll token. Sau enroll, agent nhận danh tính dài hạn.

Ref: session record cho thấy enroll token là luận điểm chính khi so sánh với SSH key root -- token phạm vi hẹp, dùng một lần, thu hồi được từ Othela, không cho ai quyền vào node. Key SSH root thì cho tất cả. Khác biệt cốt lõi: *bi mat thu hoi duoc, khong tich tu, mat thi khong mo duoc cua nao*.

- [ ] One-time enroll token: Othela sinh token, agent gọi về đăng ký (tham khảo Portainer Edge agent).
- [ ] Othela endpoint middleware verify token cho mọi Agent API.
- [ ] Agent cấu hình và gửi header Authorization kèm token đã nhận sau enroll.
- [ ] Thu hồi và xoay vòng danh tính agent sau enroll. Cần có trước khi fleet > 1 node.
- [ ] Xử lý khi node bị đánh cắp: cơ chế revoke identity từ Othela.

### 3.5. Làm việc với systemd (Agent runtime)

Helvilette lấy systemd làm *runc + containerd* của mình để giao tiếp với OS. Agent phải chạy ổn định dưới systemd trước khi có bất kỳ feature nào khác.

- [ ] Systemd unit files: `othela.service`, `helvilette-agent.service`.
- [ ] Cấu hình `Restart=on-failure`, `RestartSec`, `StartLimitBurst` trong unit file.
- [ ] Agent ghi `last_run_summary.json` trên đĩa node (tham khảo Puppet `last_run_summary.yaml`). Cho phep `h8e status` chạy được ngay cả khi Othela chết.
- [ ] Validate agent hoạt động đúng sau systemd restart, stop, reload.

### 3.6. Reconciliation Loop (Drift Detection)

Vòng lặp phát hiện drift, học từ Kubelet PLEG: poll + diff, level-triggered. Sự kiện chỉ là gợi ý, sự thật luôn là kết quả so sánh toàn phần. Không bao giờ dựng trạng thái bằng cách cộng dồn sự kiện.

Ba nguồn kích hoạt đổ vào cùng một hàm, không có đường tắt bỏ qua check mode:

1. **Git thay đổi** -- poll Gitea, so commit SHA. Interval 30-60s.
2. **Resync định kỳ** -- chạy `ansible-playbook --check --diff` dù git không đổi. Bắt drift do người sờ vào máy. Interval 10-30 phút (tham khảo Puppet/Chef `runinterval` = 1800s). Cần đo thời gian thực tế một lần check mode trước khi chốt interval.
3. **Operator gọi tay** -- `helvilette sync now`.

- [ ] Implement reconciliation loop với 3 trigger sources.
- [ ] Cache kết quả check lần trước. Cache **chỉ dùng để quyết định hiển thị** ("drift mới" vs "còn từ lần trước"), tuyet doi không dùng để quyết định *có chạy check hay không*. Nhầm chỗ này là mất tính level-triggered.
- [ ] Splay/jitter ngẫu nhiên khi poll. Thêm ngay từ đầu, dù chỉ có 1 node. Thêm sau sẽ đau khi fleet lớn (50 node cùng poll = DDoS Gitea/Othela mỗi nửa tiếng).
- [ ] Bật Ansible fact cache với TTL ngắn hơn resync interval. `gather_facts` chiếm phần lớn thời gian mỗi lần resync, node 1 vCPU sẽ nóng vô ích nếu không cache.
- [ ] So sánh state hiện tại với desired state. Nếu có drift, Agent báo cáo `DriftDetected` event về Othela.

### 3.7. Health Probes cho managed services

Phạm vi probe được chốt qua tranh luận trong session record: **probe chỉ để phát hiện thứ vẫn thở nhưng đã hỏng.** Không phải bản sao của systemd.

Hai vấn đề, hai chỗ giải quyết:
- Process chết mà không được restart -> sửa unit file, đó là **drift**, thuộc về vòng lặp Ansible. Unit file thiếu `Restart=` phải hiện trong báo cáo với dấu `~`.
- Process sống mà chạy sai -> probe. Systemd mù ở chỗ này, không ai lấp được.

Mặc định khi probe báo unhealthy: **báo và ghi, không tự restart**. Restart là opt-in từng service, kèm `--reason`. Nguyên tắc detection-only áp dụng nhất quán ở mọi tầng.

Khi được dạy remediation: operator chỉ tới một playbook cụ thể (không phải cờ `restart: true`). Play đó phải đã chạy dry-run ít nhất một lần và pass trước khi được phép gắn vào probe. Không có thứ gì chạy tự động mà chưa từng được nhìn thấy chạy khô.

Pattern ONESHOT cho remediation: chạy một lần, kiểm tra lại, xanh thì im, không xanh thì **halt va im luon**. Sau halt, agent không được tự thoát trạng thái đó. Chỉ operator gỡ kèm `--reason`. Tự hết hạn rồi thử lại = vòng lặp chậm, loại tệ nhất.

Tên "readiness" cần đổi: trong K8s nó nghĩa là "đừng route traffic vào đây", Helvilette không có bộ định tuyến traffic. Cái thực sự cần là biết service ổn định trước khi chuyển sang node kế tiếp trong rolling update -- đó là **điều kiện cổng** (gate condition), không phải readiness.

- [ ] Mở rộng `pkg/manifest/types.go` để parse probes section từ `helvilette.yml`.
- [ ] Support **liveness probe** cho systemd services (HTTP get, TCP socket, Exec).
- [ ] Support **gate condition** (thay cho "readiness") cho rolling update sequencing.
- [ ] Sau khi probes có types: bật `yaml.Decoder.KnownFields(true)` trong `ParseFile` (xem ADR-0002, Follow-up work).
- [ ] Agent định kỳ kiểm tra sức khỏe service, độc lập với vòng lặp Ansible.
- [ ] Implement ONESHOT + halt pattern cho remediation playbook.
- [ ] `RestartFailureBackOff` khi remediation thất bại.

### 3.8. Structured Logging (cho người và máy đọc)

Mô hình OX: `+ - ~ ↻ ?`, lưu thật giàu, hiện thật gọn. Ký hiệu `?` (không biết) không bao giờ bị giấu.

Nguyên tắc cốt lõi (Concorde session): **lưu giàu, hiển thị nghèo.** Một nguồn sự thật duy nhất (event stream JSONL), mọi thứ người nhìn thấy là *view* tính ra từ nó. "Respect the machine" = không vứt gì. "Operator above all" = mặc định không ai phải nhìn thấy nó.

Log phải phục vụ **ba** đối tượng (không phải hai):
- **Người đang ngồi nhìn ngay lúc này** -- "nó đang làm gì, còn bao lâu". Cần dòng chảy, đọc bằng mắt.
- **Người đọc lại lúc 3 giờ sáng** -- "đã đổi gì, có ổn không". Cần tóm tắt, không cần bản ghi đầy đủ.
- **Cỗ máy** -- `why`, trạng thái Othela, audit log. Cần cấu trúc, truy vấn được, bền.

Đường ống log:
1. Ansible -> Agent qua **callback plugin** (không parse text output). Mỗi sự kiện = 1 dòng JSON. Tham khảo lược đồ sự kiện của `ansible-runner` (Red Hat).
2. Agent ghi thẳng xuống đĩa: `/var/lib/helvilette/runs/<job-id>/events.jsonl`. Append-only.
3. Agent vừa nhận sự kiện, vừa dựng bản tóm tắt trong bộ nhớ.
4. Chạy xong, Agent ghi `summary.json` (~2KB) và chỉ gửi *cái này* về Othela. File JSONL nằm lại trên node.

Bản tóm tắt phải dịch từ task sang **thay đổi trên máy**: `template`/`copy`/`file` -> đường dẫn, `service`/`systemd` -> unit + hành động, `apt`/`yum` -> gói + phiên bản cũ/mới. Khoảng 15 module phủ 90% playbook thực tế.

Luật trung thực: **không bao giờ hiển thị tóm tắt sạch cho lần chạy có chứa task không dự đoán được.** Nếu playbook có `shell` task, tóm tắt phải kết thúc bằng dòng `?`. Nói dối ở playbook cần soi kỹ nhất = mất niềm tin vĩnh viễn.

Phải nói cả cái KHÔNG đổi: `44 task · 4 thay đổi · 40 đã đúng sẵn · 0 lỗi`. Thiếu dòng này thì lúc 3h sáng không phân biệt được "đã hội tụ" vs "không chạy được".

Agent cũng ghi 1 dòng tóm tắt vào **journald/syslog**: `helvilette[1042]: job 7f3a-... commit a3f9c21 · 4 changed, 0 failed`. Kể cả khi Alloy chưa cài, `/var/lib/helvilette` bị xóa, `journalctl -u helvilette` vẫn kể được câu chuyện. Mức sàn không phụ thuộc gì.

**Helvilette không vận chuyển log.** Agent ghi file chuẩn, công cụ quan sát có sẵn (Alloy, etc.) làm nốt. Không có log shipping, buffer, retry từ Agent về Othela.

Cảnh báo Loki label: không đưa `job_id`/`commit` lên làm label (cardinality vô hạn -> Loki chết). Label: `node`, `playbook`, `status`. Field trong dòng JSON: `job_id`, `commit`, `task`, `module`, `changed`. Đường dẫn file phải cố định và đoán được.

- [ ] Viết Ansible callback plugin cho Helvilette Agent. Tham khảo `ansible-runner` event schema.
- [ ] Thiết kế output format cho terminal (OX symbols: `+ - ~ ↻ ?`).
- [ ] Thiết kế structured JSON schema cho machine consumption.
- [ ] Parse sự kiện callback thành OX symbols. Dịch module name -> thay đổi trên máy (file path, unit name, package version).
- [ ] `?` cho task không check-safe (không dry-run trung thực được, tương tự Puppet `exec` cần `unless`/`onlyif`/`creates`).
- [ ] Ghi `summary.json` trên node (phục vụ `h8e status`/`h8e why` khi Othela chết).
- [ ] Gửi summary về Othela (phục vụ fleet-wide view). Full JSONL chỉ gửi khi có người hỏi.
- [ ] Ghi 1 dòng tóm tắt vào journald/syslog mỗi lần chạy.
- [ ] Ba khung nhìn log: `h8e logs <job>` (tóm tắt mặc định), `h8e logs <job> --tasks` (mỗi task 1 dòng, gấp gọn), `h8e logs <job> --json` (JSONL gốc). Khi hỏng: tự bung task lỗi + N task trước nó (tham khảo GitHub Actions fold/unfold). Cờ `-C`/`--context` cho số task hiển thị quanh chỗ lỗi (mượn trí nhớ cơ bắp từ `grep`/`diff`).

### 3.9. Agent behavior khi Othela chết

Câu hỏi lớn nhất của mô hình pull. Câu trả lời quyết định schema lưu trữ.

Vòng phụ thuộc cần chốt: Helvilette cài observability stack (Alloy, etc.) -> Alloy chuyển log Helvilette -> khi Helvilette hỏng, đường nhìn vào Helvilette cũng hỏng. Nên: `h8e logs` và `h8e why` phải chạy từ file trên node, không cần Loki, không cần Othela, không cần mạng. Loki là tiện lợi; file trên đĩa là sự thật.

- [ ] Quyết định: agent vẫn chạy check bình thường khi mất kết nối Othela, hay dừng lại?
- [ ] Nếu vẫn chạy: xếp báo cáo vào hàng đợi local, flush khi Othela sống lại.
- [ ] Nếu dừng: ghi rõ lý do vào `last_run_summary.json`, `h8e status` phải cho thấy "Othela unreachable since ...".

### 3.10. Ngữ nghĩa Job

Chốt từ session: đây là điều kiện sống còn cho bài k3s-ansible. Agent nhận job, chạy xong, rồi chết trước khi báo cáo. Khởi động lại thì sao?

- [ ] Job phải có ID duy nhất. Agent ghi "đã bắt đầu job X" xuống đĩa **trước** khi chạy.
- [ ] At-most-once semantics: khi khởi động lại, agent phải nhận biết job dở dang. Không được im lặng chạy lại (apply hai lần playbook k3s = phá cụm).
- [ ] Agent tự cắt chân mình: playbook nghịch iptables/systemd-networkd khiến agent mất kết nối Othela giữa lúc chạy. Khi mạng hồi phục, agent phải báo cáo kết quả job đã chạy xong lúc mất mạng. Không mất kết quả, không chạy lại, không treo vĩnh viễn.
- [ ] Trạng thái job dở dang: `h8e status` hiển thị `⏸ mất liên lạc lúc task N/M · job <id> chưa kết thúc`.

### 3.11. Preflight / Preview (thay vì "plan")

Đừng gọi là "plan" -- gọi là **preflight** hoặc **preview**. Nếu người dùng kỳ vọng độ chính xác kiểu Terraform mà nhận được Ansible check mode, lần bất ngờ đầu tiên giết niềm tin vĩnh viễn.

Ansible `--check` **bỏ qua** `command`/`shell`/`raw` theo mặc định (báo "skipped"). Hỏng cả hai chiều:
- Xanh giả: check bảo không đổi, apply xóa sạch node.
- Đỏ giả: task sau phụ thuộc `register` của task bị skip -> check fail -> agent halt -> gọi người dậy 3h sáng vì lý do không tồn tại.

Cơ hội lớn nhất: **đo độ tin cậy preview** (chưa ai trong thế giới Ansible làm). Quét playbook, đếm task không check-safe, in kèm preview.

Tách bạch hai trạng thái: *check chạy và thất bại* (halt đúng) vs *check không chạy được* (halt sai -- báo cáo, không chặn).

Thông tin nguy hiểm: in ra dòng lệnh nguy hiểm, không in phần trăm. Ngưỡng phần trăm là thước đo sai -- 6 task/40 = 15% nghe lành, nhưng 1 trong 6 là `rm -rf /var/lib/postgresql` thì 15% vô nghĩa. In thẳng danh sách task không check-safe. Đánh dấu đỏ task chứa `rm`, `dd`, `mkfs`, `truncate`, `DROP`, hoặc chạm đường dẫn bảo vệ.

File `.previewed`: agent sinh, Othela lưu. Phải gắn với cặp (hash trạng thái node, commit SHA). Apply phải từ chối nếu một trong hai đã đổi kể từ lúc preview.

Không dùng lock; dùng **lease có TTL** tự hết hạn. Agent giữ lock rồi chết trên VPS mạng chập chờn xảy ra hằng tuần. `terraform force-unlock` nổi tiếng khó chịu vì lý do đó.

- [ ] Implement preflight/preview command (`h8e preview <node>`).
- [ ] Chấm điểm độ tin cậy preview: N/M task dự đoán được + danh sách task không check-safe.
- [ ] Heuristic đánh dấu đỏ task phá hủy (regex pattern cho `rm`, `dd`, `mkfs`, etc.).
- [ ] Ngưỡng cấu hình theo repo trong `helvilette.yml` (để shop 60% `shell` không bị cảnh báo đến mức tắt hẳn).
- [ ] File `.previewed` sinh bởi agent, gắn với (node state hash, commit SHA). Apply từ chối nếu state đã đổi.
- [ ] Lease có TTL thay vì lock cho trạng thái đang preview/apply.

### 3.12. h8e CLI -- OX Commands

Tên `h8e` -- đúng 8 ký tự bị lược bỏ giữa h và e. Học CLI pattern từ Terraform (động từ, không cờ; plan và apply tách riêng; không có gì nguy hiểm xảy ra mà không hỏi). Học khoảnh khắc cài đặt từ Ansible (chạy được ngay, không cần `init`). Mẫu mực cho `why`/`status`: `git status` -- mỗi lần in trạng thái, in luôn lệnh kế tiếp.

- [ ] `h8e why <node>`: chạy **trên node**, đọc từ trạng thái cục bộ, không cần Othela. Trả lời: cái gì đổi, ai quyết, vì sao, lệnh nào hoàn tác. Output mẫu:
  ```
  Node này đang ở commit a3f9c21, áp dụng 47 phút trước.
    Người đẩy: khoa — "tăng nginx worker_connections"
    Thay đổi trên node: nginx.conf, restart nginx.service
    Trạng thái tốt gần nhất: 8e21b04 (3 ngày, 0 lỗi)
    Quay lại:  h8e pin 8e21b04
  ```
- [ ] `h8e pause --reason "..."`: tạm dừng agent trên node, lý do bắt buộc. Tham khảo `puppet agent --disable "reason"`. Người tiếp theo gõ lệnh đọc được dòng lý do.
- [ ] `h8e freeze --reason "..."`: đóng băng toàn fleet từ Othela. Một lệnh duy nhất khi commit thảm họa.
- [ ] `h8e unfreeze`: mở băng fleet.
- [ ] `h8e fleet`: trạng thái tổng quan fleet. Không tiến cử, không tự lên tiếng, chỉ trả lời. Output mẫu:
  ```
  34 node · 34 đang báo danh
    Chưa khai log_sink:                11 node
    Chưa từng apply thành công:         2 node (vps-01, vps-02)
    Đang đóng băng:                     0
    Lệch so với repo:                   3 node
  ```
- [ ] `h8e apply <node|--group>`: apply thủ công. In preflight rồi dừng chờ `yes`. Không có gì nguy hiểm xảy ra mà không hỏi.
- [ ] `h8e apply --force --reason "..."`: cửa thoát hiểm. Lý do bắt buộc, tự sinh dòng cho immutable log. Không ai dán vào script mà không viết vì sao.
- [ ] `h8e backup` / `h8e restore <file>`: native backup. Ra một file duy nhất chứa cả dữ liệu và khoá. Không có chuyện "nhớ sao lưu thêm file này".
- [ ] `h8e tunnel <node>`: mở Chisel tunnel về node, tự hết hạn sau timeout. Không mở port 22.
- [ ] `h8e status`: đọc `last_run_summary.json` trên đĩa node. Chạy được khi Othela chết.
- [ ] `h8e sync now`: kích hoạt reconciliation loop ngay lập tức.
- [ ] `h8e uninstall`: agent biến mất sạch, managed services vẫn chạy. Lời hứa zero lock-in, chứng minh chứ không viết.

### 3.13. Production Readiness
- [x] Health check endpoints (/healthz, /readyz).
- [x] Graceful shutdown handling cho Othela và Agent.

### 3.14. `helvilette.yml` -- Constraints

Ba luật bất khả xâm phạm cho file config:
1. **Không có `helvilette.yml` thì vẫn phải chạy được.** Mặc định: cả repo, mọi host, apply thủ công. File chỉ để tinh chỉnh.
2. **Repo phải vẫn chạy được bằng `ansible-playbook` trần.** `helvilette.yml` là file lạ mà Ansible bỏ qua.
3. **Mọi trường đều phải suy ra được hoặc bỏ được.** File tối thiểu <= 10 dòng.

Thước đo: file phải tự trả tiền cho mình trong **tuần đầu tiên** -- cài agent, gõ `h8e fleet`, lần đầu nhìn thấy trạng thái thật của fleet. Chưa apply gì, chỉ nhìn thấy.

- [ ] Validate `helvilette.yml` optional: Othela chạy được không file, mặc định sane.
- [ ] Kiểm tra repo vẫn chạy bằng `ansible-playbook` thuần khi có `helvilette.yml`.

---

## 4. Medium Priority Backlog (Nice-to-Have / Post-MVP)

Sau khi hệ thống lõi hoạt động hoàn chỉnh, tiến hành các tính năng bổ sung.

### 4.1. Giao thức Agent-Othela -- Versioning & Efficiency

- [ ] **Giao thức có số phiên bản tường minh** trong mọi thông điệp, từ commit đầu tiên. Tương thích ngược ít nhất 1 minor version. Không có -> "hot-patch" chỉ là "downtime có kế hoạch".
- [ ] **ETag cho long-poll.** Portainer dùng ETag header + exponential backoff có jitter -> ~324 B/s mỗi agent. Helvilette đang trả full payload mỗi lần. ETag là khác biệt giữa 50 node và 5000.
- [ ] **Fail-safe poll interval.** Nếu tất cả poll interval bị set về 0, agent kích hoạt fail-safe và tự đặt ping về mặc định (tham khảo Portainer Edge Agent). Server cấu hình sai không được phép làm cả fleet biến mất.

### 4.2. Chisel Socket Stream

Kết nối agent -> Othela qua Chisel tunnel. Giải quyết NAT, edge, 4G. Để sau khi enroll token đã ổn định, vì Chisel là đường thoát hiểm cho lỗi -- mà chưa biết lỗi trông thế nào.

- [ ] Tích hợp Chisel client vào agent.
- [ ] Tích hợp Chisel server vào Othela.
- [ ] Fallback khi Chisel tunnel gãy.

### 4.3. Ansible Playbook & Bash Script cài đặt / gỡ cài đặt

Tham khảo `get.k3s.io`. Script bootstrap cần 4 việc:
1. Tải binary agent đúng kiến trúc (amd64/arm64 -- đã hứa Raspberry Pi), verify checksum.
2. Ghi systemd unit + file config.
3. Enroll vào Othela -- one-time token, agent tự gọi về đăng ký.
4. Ghi `helvilette-agent-uninstall.sh` (tham khảo k3s-uninstall.sh -- nói với người cài "anh rút ra được bất cứ lúc nào", đúng tinh thần zero lock-in).

Đừng viết script bootstrap trước khi Round 0 chạy được. Không thì sẽ có script cài rất mượt một thứ chưa ai biết có chạy không.

Cam kết README: một binary, một file config, một SQLite. Không container bắt buộc, không cụm, không hàng đợi, không Redis.

- [ ] Bash install script (`get.helvilette.io`).
- [ ] Ansible playbook cho bootstrap (Helvilette mà không có Ansible đi bootstrap Ansible + Helvilette là sỉ nhục).
- [ ] Uninstall script ghi tự động lúc cài.
- [ ] Cấu hình qua biến môi trường `INSTALL_HELVILETTE_*` để script chạy không tương tác.
- [ ] Bài test CI: `h8e uninstall` trên node -> agent biến mất sạch, managed services vẫn chạy.
- [ ] Bài test CI: `h8e backup` -> đập Othela -> `h8e restore <file>` trên máy trắng -> agents tự tìm về, lịch sử còn nguyên.

### 4.4. Othela Playbook / Repo Management (Multi-repo support)
- [ ] pkg/git/repo.go & watcher.go: Othela tự động sync/track các repos.
- [ ] API Endpoints đăng ký, list, và manual sync Repos (POST /api/v1/repos).

### 4.5. Webhook Triggers
- [ ] Othela lắng nghe Webhook (từ GitHub/Gitea/GitLab) khi có git push.
- [ ] Khi nhận trigger, Othela lập tức notify/invalidate cache các Agents liên quan mà không phải đợi hết Poll Interval.

### 4.6. Vault / Secret Integration
- [ ] Mở rộng pkg/manifest/types.go để parse vault section từ helvilette.yml.
- [ ] Support type: exported (đọc secret từ ENV của Othela host).
- [ ] Support type: hashicorp_vault (đọc secret từ HashiCorp Vault API).
- [ ] Agent nhận vault password file path từ Job, inject vào ansible-playbook --vault-password-file.

### 4.7. Scheduled Playbook Runs
- [ ] Hỗ trợ Cron-like schedule để tự động trigger job từ phía Othela thay vì chỉ chạy 1 lần.

### 4.8. README -- Bảng so sánh cài đặt AWX vs Helvilette

Không phải bảng so sánh tính năng. Chỉ bảng cài đặt. Người đã từng dựng Kind chỉ để thử AWX hiểu ngay:

| | AWX | Helvilette |
|---|---|---|
| Yêu cầu | Cụm Kubernetes | Một binary |
| Backup | Tự viết script pg_dump, nhớ sao lưu SECRET_KEY riêng | `h8e backup` (một file duy nhất) |
| Khôi phục | Nhiều bước, dễ mất khoá | `h8e restore <file>` |

Đừng quên bảng trạng thái tính năng: ✅ đã chạy / 🚧 đang làm / 📋 đã thiết kế. Không bán thứ code chưa có.

- [ ] Viết bảng so sánh cài đặt cho README.
- [ ] Thêm bảng trạng thái tính năng (maturity markers) vào README.
- [ ] Thêm `ansible-pull` vào bảng so sánh -- đặt ở dòng đầu. Đó chắc chắn là câu hỏi đầu tiên.

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

### 5.3. Open-core boundary

Luật chia free/paid:
> **Miễn phí**: mọi thứ một đội 1-2 người cần để vận hành an toàn.
> **Trả tiền**: mọi thứ chỉ cần thiết vì có *người khác* tham gia.

Cam kết: *bất cứ thứ gì team 1-2 người cần để chạy production an toàn thì miễn phí, vĩnh viễn, không bao giờ chuyển lên paid.*

Đường phân chia:
- Free: mTLS, enrollment token, 1 admin account, lịch sử job, drift, fleet status, plan/preflight, rollback, freeze, scheduled runs, webhook.
- Paid: RBAC/Teams/Orgs, quy trình phê duyệt, SSO/LDAP/SAML, immutable audit log, access review export, FIPS 140 build.

Không bao giờ đặt paywall lên **API hoặc quyền xuất dữ liệu**. Bản miễn phí phải cho lấy toàn bộ dữ liệu ra. Bán thứ xây trên dữ liệu, không bán quyền chạm vào dữ liệu.

Giữ code thương mại ở thư mục/repo riêng ngay từ đầu. Lõi Apache-2.0 sạch. Trộn vào rồi gỡ = khổ sai.

- [ ] Tách thư mục code thương mại ra repo riêng từ đầu.
- [ ] Viết tài liệu cam kết free/paid boundary.

### 5.4. CLA (Contributor License Agreement)

Giữ Apache-2.0, thu hút người dùng. Dựng CLA **trước PR đầu tiên từ người ngoài**. Chừng nào còn là chủ sở hữu bản quyền duy nhất, quyền relicense vẫn mở. Nhận PR không CLA = mất quyền đơn phương relicense.

- [ ] Dựng CLA trước khi merge PR từ contributor bên ngoài.

---

## 6. Technical Debt

Các hạng mục dưới đây phát hiện khi làm ADR-0002 và issue #13, chưa xử lý.
Chỉ mục 6.1 có issue riêng. Với các mục còn lại, phần mô tả ở đây là nguồn context duy nhất.

### 6.1. Node khớp nhiều nodeGroup thì chỉ nodeGroup đầu tiên được chạy

Issue: #15. Mức độ: cao. Đây là lỗi đúng nghĩa, không phải nợ kỹ thuật thuần tuý.

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

---

## 7. Design Decisions (chốt từ session records)

Các quyết định thiết kế sau được chốt từ hai session:
- `Claude-Naughtian projects and national tech infrastructure-20260829-1739.md`
- `Claude-Helvilette_concorde_test-20260829-1740.md`

Ghi lại ở đây để không bị rơi rớt.

### 7.1. Triết lý Detection-first, Remediation opt-in

Helvilette chọn phương án ít gây thiệt hại nhất: mặc định chỉ phát hiện drift, báo to (detection-only). Remediation yêu cầu:
- Operator dạy Helvilette cách fix bằng playbook lũy đẳng.
- Opt-in **từng service một**, kèm `--reason`.
- Playbook chữa cháy phải đã chạy dry-run ít nhất một lần và pass.

Triết lý: "playbook của anh lũy đẳng thì việc này an toàn, không lũy đẳng thì đằng nào cũng chết, Helvilette chỉ khiến nó chết đều đặn hơn thôi."

### 7.2. Operator Pattern cho non-K8s

Helvilette có hình dạng operator (observe -> compare -> act) nhưng đặt ở Layer 2 (systemd/OS). Ranh giới:
- **Làm operator** cho mọi thứ không cần bầu leader: cert renewal, log rotation, config reload, backup monitoring.
- **Không làm** failover, fencing, leader election -- bất kỳ việc nào hai node cùng ra tay sẽ hỏng không cứu được.
- **Chăm sóc người chăm sóc**: giữ cho Patroni/etcd/các công cụ HA không chết vì drift config.

### 7.3. Skill thang, không phải skill cổng

Helvilette **không đòi hỏi** trình độ cao, nó **thưởng** cho trình độ cao.
- Tầng đáy (detection-only): cài agent, nhận báo cáo drift. Không cần biết Ansible sâu.
- Tầng cao (auto-remediation): viết play lũy đẳng, biết healthcheck có nghĩa.
- Đừng viết "skill-oriented" trên trang chủ -- nhóm SMB/homelab 5-50 VM sẽ nghĩ "không dành cho mình".

### 7.4. Giá trị cốt lõi: biết trạng thái fleet

Cái mạnh nhất của K8s không phải self-healing, mà là biết trạng thái thật của toàn cụm ở một chỗ. Helvilette mang điều đó cho non-K8s. Self-healing là phần thưởng cho người leo lên, phát hiện drift là thứ khiến người ta giữ tool lại sau sáu tháng.

### 7.5. Operator Experience (OX) là trung tâm triết lý vận hành

OX không phải thứ bán. Nó là thứ khiến Helvilette **được chọn**. Tiền đến từ tầng khác chồng lên (RBAC, audit log, SSO).

Lý do các hãng lớn không làm được OX: người mua Ansible Automation Platform là VP ký hợp đồng, không phải người bị gọi dậy 3h sáng. Họ tối ưu cho khoảnh khắc ký hợp đồng. Helvilette tối ưu cho thứ Ba, 3 giờ sáng. Ở 12-50 VM, người vận hành *chính là* người mua.

Hệ quả bắt buộc: **Helvilette sẽ có ít tính năng hơn AWX, một cách cố ý.** OX đắt trên từng tính năng. Không viết ra thì sáu tháng nữa vừa đuổi tính năng vừa tự nhủ đang làm OX, mất cả hai.

Bốn ràng buộc test được:
1. Mọi thông báo lỗi phải nêu hành động kế tiếp. Grep được, kiểm tra được trong CI.
2. Thời-gian-đến-thành-công-đầu-tiên: từ lệnh cài đến apply thành công đầu tiên trên VM trắng < N phút. Biến thành bài test Testcontainers.
3. Chẩn đoán được từ node, không chỉ dashboard. `journalctl -u helvilette` phải đủ.
4. Gỡ cài đặt bằng một lệnh, thực sự sạch.

Peak OX: **tự động hóa mà bạn có thể tra hỏi và ngắt được.**

### 7.6. Triết lý thiết kế: Pháp cài đặt, Đức ngữ nghĩa

> Giấu độ phức tạp của **thiết lập**. Phơi trần **hậu quả**.

Phần Pháp (Traefik, Meilisearch, Docker): một binary tĩnh, chạy được ngay, tự khám phá, mặc định có chính kiến mạnh. Phần Đức (systemd, SUSE): khai báo tường minh, đúng đắn hơn tiện lợi, không ưa phép màu.

Cố tình quay lưng với Thung lũng Silicon: không thiết kế cho quy mô chưa có, không HA mặc định vì chưa cần, không bán dịch vụ managed.

Ông lớn không có động cơ làm OX vì nỗi đau self-host *chính là* phễu bán hàng cho bản cloud. Kẻ duy nhất không có động cơ giữ nỗi đau = kẻ không bán dịch vụ managed.

### 7.7. Agent chỉ báo cáo về chính nó

> Agent chỉ báo cáo những gì nó biết về **chính nó**. Nó không suy đoán về node.

Ví dụ: `log_sink: unset` (chưa khai báo trong config) -- không phải warning, không màu đỏ, không dấu chấm than. Operator thấy `unset`, tự hiểu. Khai `log_sink: none` thì ô biến mất.

Luật này tự động giải quyết mọi trường hợp tương tự: backup, cảnh báo, giám sát. Khi Helvilette *tự nói* không ai yêu cầu, nó thành Clippy. Clippy sinh từ "đoán ý bạn".

### 7.8. Hành động nguy hiểm luôn để lại giấy nhắn

Quy tắc nhất quán: `pause --reason`, `force --reason`, `freeze --reason`. Không bao giờ có hành động nguy hiểm trơ trọi. Ba lợi ích: không ai dán vào script, tự sinh dòng cho audit log, `h8e why` trả lời được.

### 7.9. Ansible không có trí nhớ -- đó là giá trị Helvilette tạo ra

Mỗi lần `ansible-playbook` chạy là sự kiện mất trí. Không ai trả lời được: lần cuối apply là khi nào, node đang ở commit nào, 6 tuần trước có ai sửa tay không, node nào chưa từng apply thành công.

Helvilette có agent thường trú ghi lịch sử tại chỗ. Trí nhớ rơi ra từ kiến trúc, không phải xây thêm. **Đó là thứ đáng để học một file mới.**

### 7.10. Đánh đổi trung tâm: vòng lặp mà không trả thuế DSL

Chef/Puppet có reconciliation loop từ 2009, nhưng đi kèm DSL. Ansible thắng bằng cách vứt agent, trả giá = mất vòng lặp. Helvilette tách vòng lặp ra khỏi engine -- chưa ai làm.

Rủi ro: Ansible idempotent *theo quy ước*. Module thì ổn, nhưng `command`/`shell`/`changed_when` bịa. Chạy 48 lần/ngày thì mọi chỗ không idempotent nổ ra trên toàn fleet. Đây là câu hỏi kỹ thuật sâu nhất, sâu hơn auth, và đáng dò tìm ở alpha.

### 7.11. Portainer Edge Agent là tham chiếu kiến trúc

Edge Key = một chuỗi base64, bên trong: URL Portainer, địa chỉ tunnel server, fingerprint, mã định danh môi trường. Key dùng một lần để gắn danh tính, sau đó không tái sử dụng. Fingerprint giải vấn đề "http trần" mà không cần CA.

Credential phù du theo mặc định: sau 5 phút không hoạt động, tunnel đóng, credential thu hồi.

### 7.12. Bài thi tốt nghiệp 1.0.0: k3s-ansible

Không duyệt dựa trên lần chạy thành công. Duyệt dựa trên **bốn lần chạy hỏng**:

1. **Chạy sạch**: 3 node (server + 2 agent). Preflight phải tự thú nhận task `?`. Nếu báo xanh mượt cho k3s-ansible thì đang nói dối.
2. **Rút phích giữa chừng**: tắt nguồn agent giữa job. Agent khởi động lại **không** tự chạy lại. Báo trạng thái dở dang, chờ người quyết.
3. **Playbook hỏng giữa fleet**: agent hỏng -> dừng, không đụng tới agent khác. 60 giây phải biết dòng nào hỏng mà không mở repo.
4. **Agent tự cắt chân mình**: playbook nghịch iptables, agent mất kết nối giữa lúc chạy. Mạng hồi phục -> báo cáo kết quả. Không mất, không chạy lại.

Sau đó: backup -> đập Othela -> restore trên máy trắng -> agents tự tìm về. Cuối cùng: `h8e uninstall` -> agent biến mất, k3s vẫn chạy.

> Cụm k3s lên được thì **chưa đủ**. Cụm k3s *hỏng* mà vẫn biết chính xác chuyện gì xảy ra, và sửa được -- **đó** mới là 1.0.0.

Chia mức:
- Mức 1: k3s single node. Khả thi tháng này.
- Mức 2: 1 server + 2 agent. Cần điều phối thứ tự, trung chuyển token qua Othela. Đây là 1.0.0 thật.
- Mức 3: HA multi-server + etcd. Dành cho 2.0.

### 7.13. Concorde Test -- chuẩn hiệu năng Naughtistack

Định nghĩa: một cửa sổ thời gian hữu hạn, một khối lượng công việc cố định, hệ thống phải giữ được nhịp bên trong cửa sổ -- không nhanh hơn, không chậm hơn.

Concorde test chỉ áp dụng khi hệ thống có **thời hạn cưỡng bức**: chậm gây ra loại hỏng *khác*, không chỉ là khó chịu.

Helvilette ở 12-50 VM **chưa cần** Concorde thật (apply mất 20 phút thay vì 8, không có gì hỏng). Cửa sổ thật của Helvilette là **thời gian phản ứng sự cố**: commit hỏng ra fleet, bao lâu để đóng băng và quay lui.

Bài test Helvilette nên chia:
- **k3s test** -- đúng đắn khi hỗn loạn. Cần ngay, cho 1.0.0.
- **Hot-patch test** -- thay Othela giữa lúc bay (restart Othela khi 50 agent đang chạy job). Nhỏ, làm được sớm.
- **Concorde đầy đủ** -- chỉ có nghĩa khi 500 node hoặc khi định nghĩa lại cửa sổ theo thời gian phản ứng.

Cách chạy 500 node: viết agent giả (binary nhỏ nói đúng giao thức long-poll, ngủ ngẫu nhiên mô phỏng playbook, báo cáo). 500 agent giả trong container + 5 agent thật trên VM thật.

Ba mối nối phải đặt ngay: giao thức có version number, trạng thái job trong SQLite, agent ghi checkpoint xuống đĩa trước mỗi pha.

> Helvilette được đánh giá bằng cách nó hỏng, không phải bằng cách nó chạy.
