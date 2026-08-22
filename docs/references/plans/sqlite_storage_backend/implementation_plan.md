# SQLite Storage Backend cho Othela

Tách storage layer ra khỏi `server.go` bằng interface, sau đó implement SQLite adapter. Chia làm 2 phase: Phase A tách interface (refactor thuần tuý, không đổi hành vi), Phase B cắm SQLite vào.

## Quyết định dựa trên k3s

Ba câu hỏi mở đã được giải đáp bằng cách tham khảo k3s:

### 1. SQLite driver: `mattn/go-sqlite3` (CGO)

k3s/Kine dùng `mattn/go-sqlite3`. Lý do: battle-tested, hiệu suất tốt hơn, production-grade. Nhược điểm là cần C compiler khi build, nhưng k3s cũng chấp nhận trade-off này.

### 2. DB file path: `{data-dir}/server/db/state.db`

k3s lưu SQLite tại `/var/lib/rancher/k3s/server/db/state.db`. Helvilette sẽ theo pattern tương tự:
- Flag `--data-dir` (đã có, default `helvillette/othela/data/playbooks`)
- DB path sẽ là `{data-dir}/server/db/state.db`
- Tự tạo thư mục nếu chưa có

### 3. currentJob / playbooks -- giữ nguyên trong Server struct

k3s dùng Kine để lưu **toàn bộ** Kubernetes state dưới dạng key-value (vì nó giả lập etcd). Helvilette không có nhu cầu này. `currentJob` và `playbooks` là runtime state được build từ filesystem (playbook loader scan) và manifest matching -- không phải data cần persist. Chỉ **kết quả** (nodes, reports) mới cần persist.

Tóm lại: k3s persist everything vì nó thay thế etcd. Helvilette chỉ persist nodes + reports vì playbook source-of-truth nằm ở Git repo.

---

## Proposed Changes

### Phase A: Tách Storage Interface

Mục tiêu: Định nghĩa storage interface trong `pkg/storage/`, di chuyển `InMemoryNodeRegistry` thành adapter implement interface mới, thêm `ReportStore` interface. Server struct dùng interface thay vì concrete type. Toàn bộ test hiện tại phải pass không đổi.

---

#### [NEW] [storage.go](file:///home/stella/workspace/naughtian-helvilette/pkg/storage/storage.go)

Định nghĩa 2 interface chính:

```go
// pkg/storage/storage.go
package storage

import "helvilette/pkg/types"

// NodeStore quản lý thông tin node (agent) đã đăng ký.
type NodeStore interface {
    Register(nodeID string, labels map[string]string) error
    GetLabels(nodeID string) (map[string]string, bool)
    IsRegistered(nodeID string) bool
}

// ReportStore quản lý execution report từ Agent gửi về.
type ReportStore interface {
    Save(report types.Report) error
    List() ([]types.Report, error)
}
```

Giữ interface nhỏ nhất có thể -- chỉ bao gồm method mà `server.go` hiện tại đang dùng. Mở rộng sau khi cần (ví dụ `ListNodes`, `GetReportsByNode`).

---

#### [NEW] [memory.go](file:///home/stella/workspace/naughtian-helvilette/pkg/storage/memory.go)

Di chuyển `InMemoryNodeRegistry` từ `server.go` sang đây, đổi tên thành `MemoryNodeStore`. Thêm `MemoryReportStore`.

---

#### [NEW] [memory_test.go](file:///home/stella/workspace/naughtian-helvilette/pkg/storage/memory_test.go)

Unit test cho `MemoryNodeStore` và `MemoryReportStore`.

---

#### [MODIFY] [server.go](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/server.go)

1. **Xóa** `NodeRegistry` interface, `InMemoryNodeRegistry` struct và tất cả method (dòng 24-62).
2. **Thay đổi** `Server` struct:
   - `nodeRegistry NodeRegistry` --> `nodeStore storage.NodeStore`
   - `reports []Report` --> `reportStore storage.ReportStore`
   - Xóa mutex liên quan tới reports (mutex giờ nằm trong adapter)
3. **Cập nhật** tất cả constructor dùng `storage.NewMemoryNodeStore()` và `storage.NewMemoryReportStore()`.
4. **Cập nhật** handler: dùng `s.nodeStore` và `s.reportStore`.

---

#### [MODIFY] Test files: không đổi logic, chỉ cập nhật nếu `GetReports()` đổi signature.

---

### Phase B: SQLite Adapter

---

#### [NEW] [sqlite.go](file:///home/stella/workspace/naughtian-helvilette/pkg/storage/sqlite.go)

Implement `NodeStore` và `ReportStore` bằng SQLite (`mattn/go-sqlite3`).

Schema:

```sql
CREATE TABLE IF NOT EXISTS nodes (
    node_id    TEXT PRIMARY KEY,
    labels     TEXT NOT NULL DEFAULT '{}',
    registered DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id     TEXT NOT NULL,
    job_id      TEXT NOT NULL,
    status      TEXT NOT NULL,
    task_logs   TEXT NOT NULL DEFAULT '{}',
    reported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

#### [NEW] [sqlite_test.go](file:///home/stella/workspace/naughtian-helvilette/pkg/storage/sqlite_test.go)

Test với DB tạm (`t.TempDir()`).

---

#### [MODIFY] [main.go](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/main.go)

Thêm logic: khi `--data-dir` được set, tạo `{data-dir}/server/db/state.db` và khởi tạo `SQLiteStore`. Inject vào Server.

---

#### [MODIFY] [server.go](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/server.go)

Thêm `ServerConfig` struct + `NewServerWithConfig` constructor cho dependency injection.

---

#### [MODIFY] [go.mod](file:///home/stella/workspace/naughtian-helvilette/go.mod)

Thêm `github.com/mattn/go-sqlite3`.

---

## Verification Plan

### Automated Tests

```bash
# Phase A
go test ./pkg/storage/... -v -count=1
go test ./cmd/othela/... -v -count=1

# Phase B
go test ./pkg/storage/... -v -count=1
go test ./cmd/othela/... -v -count=1
go build ./cmd/othela/...
go vet ./...
```
