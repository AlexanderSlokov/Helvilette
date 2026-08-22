# Ý tưởng

1. Xem cách mà k3s dùng sqlite, liệu có thể dùng nó cho Helvilette?
2. Cần có một design docs, một cái database schema cho Helvilette.
3. Có thể chắc rằng Helvilette Apiserver (Helvilettte Othela) sẽ cần được refactor up thành Hexagonal. và updte changelog một task riêng trước task này chăng? Theo tôi thì là cần đó, vì sau này còn cắm vào cả etcd, postgres, MariaDB, whatever humanity have...

## Things AI Agent MIGHT wants to do:

- Ta có thể làm cái vụ kiến trúc Hexagonal không? Chắc cần cái mermaid / ascii chart để tôi xem sơ đồ hexagonal của cái api server sau khi được code?
- Nên viết một file riêng về cách k3s implement cái sqlite, khá chắc là không cần làm lớp shim dịch từ etcd -> sqlite đâu.

---

## Ý kiến của Opus (2026-08-08)

Tôi đã đọc qua codebase, đọc preplan, BACKLOG, và nghiên cứu cách k3s dùng SQLite. Đây là phân tích của tôi:

### 1. Về cách k3s dùng SQLite -- có áp dụng cho Helvilette không?

k3s dùng một project riêng tên là **Kine** ("Kine is not etcd"). Kine là một shim layer dịch API call từ etcd v3 sang SQL query (SQLite, Postgres, MySQL). Nó tồn tại vì k3s cần *giả lập* etcd API cho Kubernetes API server -- một thứ mà Helvilette hoàn toàn không cần.

**Kết luận: Không nên làm theo cách k3s.** Helvilette không có etcd API để giả lập. Othela là control plane riêng, tự định nghĩa API riêng. Việc cần làm đơn giản là thay `InMemoryNodeRegistry` và in-memory `[]Report` bằng SQLite trực tiếp. Không cần shim, không cần translation layer.

Tuy nhiên, có một điểm đáng học từ Kine: nó abstract storage bằng interface. Điều này dẫn tới câu hỏi thứ hai.

### 2. Về Hexagonal Architecture cho Othela -- có cần refactor trước không?

**Tôi đồng ý rằng nên refactor trước, nhưng không cần "full hexagonal".** Lý do:

Nhìn vào `server.go` hiện tại, Othela đã có một bước đi đúng: `NodeRegistry` đã là interface (dòng 25-29). Tuy nhiên, những thành phần khác vẫn bị coupling cứng vào `Server` struct:

- `reports []Report` -- lưu thẳng trong memory, không có interface
- `playbooks []playbook.Playbook` -- load một lần, giữ trong memory
- `currentJob Job` -- fallback job không có abstraction

Việc thay thế bằng SQLite sẽ buộc ta phải chạm vào toàn bộ logic trong `handleSync`, `handleReport`, và các constructor `NewServer*`. Nếu không tách interface trước, sẽ xảy ra một trong hai tình huống:

(a) Viết SQLite code xen vào thẳng `server.go` -- file sẽ phình to, khó test, và khi cần hỗ trợ Postgres/etcd sau này thì phải refactor lại từ đầu.

(b) Viết interface ngay trong lúc implement SQLite -- làm hai việc cùng lúc, review khó, dễ lỗi.

**Đề xuất:** Tạo một task riêng trước task SQLite, chỉ làm một việc: tách storage interface. Cụ thể:

```go
// pkg/storage/storage.go
type NodeStore interface {
    Register(nodeID string, labels map[string]string) error
    GetLabels(nodeID string) (map[string]string, bool)
    IsRegistered(nodeID string) bool
    ListNodes() ([]NodeInfo, error)
}

type ReportStore interface {
    SaveReport(report types.Report) error
    ListReports() ([]types.Report, error)
    GetReportsByNode(nodeID string) ([]types.Report, error)
}

type JobStore interface {
    SaveJob(job types.Job) error
    GetLatestJob() (*types.Job, error)
    GetJobByID(jobID string) (*types.Job, error)
}
```

Sau đó `InMemoryNodeRegistry` hiện tại trở thành `InMemoryNodeStore` implement `NodeStore`, tương tự cho Report và Job. `Server` struct chỉ giữ reference tới các interface này. Code hiện tại vẫn chạy như cũ (in-memory), nhưng khi làm task SQLite, chỉ cần viết `SQLiteNodeStore`, `SQLiteReportStore`, v.v. rồi inject vào -- không cần sửa `server.go` nữa.

Đây chính là cái lõi của Hexagonal: tách port (interface) khỏi adapter (implementation). Nhưng ta chỉ cần làm đúng cái này cho storage layer, không cần refactor toàn bộ Othela thành kiến trúc hexagonal đầy đủ với domain layer, use case layer, v.v. Ở quy mô 12-50 VM, một đội 1-2 người, kinh nghiệm của tôi là over-engineering sẽ giết dự án nhanh hơn under-engineering.

### 3. Về database schema

Schema cho SQLite nên giữ đơn giản. Dữ liệu của Helvilette hiện tại chỉ có 3 entity rõ ràng:

**nodes** -- Agent đang kết nối

| Column     | Type    | Note                        |
|------------|---------|-----------------------------|
| node_id    | TEXT PK | Unique per agent            |
| labels     | TEXT    | JSON-encoded map            |
| registered | DATETIME| Thời điểm đăng ký           |
| last_seen  | DATETIME| Cập nhật mỗi lần sync/poll  |

**jobs** -- Lịch sử job đã phân phối

| Column        | Type    | Note                     |
|---------------|---------|--------------------------|
| job_id        | TEXT PK |                          |
| repo_url      | TEXT    |                          |
| version       | TEXT    | branch/tag/sha           |
| playbook_path | TEXT    |                          |
| extra_vars    | TEXT    | JSON-encoded map         |
| created_at    | DATETIME|                          |

**reports** -- Kết quả chạy từ Agent

| Column     | Type     | Note                      |
|------------|----------|---------------------------|
| id         | INTEGER PK AUTOINCREMENT |            |
| node_id    | TEXT     | FK -> nodes               |
| job_id     | TEXT     | FK -> jobs                |
| status     | TEXT     | "Success" / "Failed"      |
| task_logs  | TEXT     | JSON blob                 |
| reported_at| DATETIME |                           |

Một vài lưu ý:
- `labels` và `extra_vars` lưu dạng JSON text. SQLite có json functions (`json_extract`, `json_each`) nên vẫn query được khi cần.
- `last_seen` trong bảng `nodes` rất quan trọng cho sau này khi làm dashboard -- biết node nào còn sống, node nào mất kết nối.
- Không cần foreign key constraint ở giai đoạn này. SQLite hỗ trợ FK nhưng phải bật bằng `PRAGMA foreign_keys = ON` mỗi connection. Để phức tạp cho sau.

### 4. Về thứ tự công việc đề xuất

1. **Task A (trước):** Tách storage interface + chuyển InMemory sang implement interface mới. Kiểm tra code hiện tại vẫn pass test.
2. **Task B (task chính):** Implement SQLite adapter, viết migration/init schema, inject vào Server.
3. **(Tùy chọn):** Viết file riêng phân tích k3s/Kine -- nhưng theo tôi, nội dung ở mục 1 trên đây đã đủ. Không cần file riêng trừ khi bạn muốn làm tài liệu tham khảo chi tiết hơn.

### 5. Về công nghệ SQLite driver

Go có hai lựa chọn chính:
- `modernc.org/sqlite` -- pure Go, không cần CGO. Deploy đơn giản hơn, cross-compile dễ.
- `github.com/mattn/go-sqlite3` -- dùng CGO, mature hơn, nhanh hơn.

Với target là edge/IoT devices, tôi **đề xuất dùng `modernc.org/sqlite`**. Lý do: không phụ thuộc vào C compiler trên máy build, binary output là static, deploy lên ARM/edge device không cần lo chiều dependency. Hiệu suất chậm hơn `mattn` khoảng 10-20% nhưng ở quy mô Helvilette (12-50 node) thì không có ý nghĩa.

### Tổng kết

- Không cần làm shim kiểu Kine. Helvilette có API riêng, chỉ cần viết SQLite adapter thẳng.
- Nên tách storage interface trước (task riêng), rồi implement SQLite sau (task riêng). Hai task nhỏ, review dễ, rollback dễ.
- Schema 3 bảng: nodes, jobs, reports. Giữ đơn giản.
- Dùng `modernc.org/sqlite` để tránh CGO.