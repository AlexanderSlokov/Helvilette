# Fix #31: Migrate Othela Logging to Structured JSON

## Nguyên tắc dẫn đường

> **Operator aboves all, but respect the machine.**

Log không phải cho developer debug. Log là cho người bị gọi dậy lúc 3 giờ sáng, SSH vào một node, chạy `journalctl -u helvilette-othela`, và cần hiểu chuyện gì đang xảy ra — **mà không cần đọc source code**.

Hệ quả cho mỗi dòng log chúng ta viết:

1. **Structured JSON** — để máy parse được (Loki, grep, jq). Plain text prefix `[ERROR]` không phải log level.
2. **Mỗi lỗi trả lời 3 câu**: hỏng cái gì, ở đâu, giờ làm gì. `"connection refused"` là lỗi tệ. `"cannot initialize SQLite at /var/lib/helvilette/othela/db/state.db: permission denied — check directory ownership or use --state-dir"` là lỗi tốt.
3. **Component tagging** — mỗi log line tự khai danh (`"component":"othela"`, `"component":"fleet-sync"`). Khi Othela và Agent chạy trên cùng host, grep theo component là cách duy nhất tách chúng.
4. **Log level phải hoạt động thật** — `--log-level=error` phải chỉ thấy error. Không phải manual `if s.debugMode` rồi `Printf("[DEBUG]...")`.

Đây là nền móng cho mọi thứ OX về sau — `helvilette why`, `helvilette pause`, preflight report. Nếu log không có cấu trúc, không có component, không có level thật, thì mọi tính năng chẩn đoán xây lên trên đều là cát.

---

## Proposed Changes

### Component 1: `pkg/log` — Thêm `SetLevel()`

#### [MODIFY] [`log.go`](file:///home/stella/workspace/naughtian-helvilette/pkg/log/log.go)

Thêm hàm `SetLevel(level string)` để CLI flags (`--log-level`) có thể connect vào zerolog global level **tại runtime**, thay vì chỉ đọc env `HELVILETTE_LOG_LEVEL` trong `init()`.

```go
// SetLevel sets the global log level from a string.
// Accepted values: "debug", "info", "warn", "error". Defaults to "info".
//
// Usage:
//   log.SetLevel("debug")  // called from CLI flag parsing in main()
func SetLevel(level string) {
    switch level {
    case "debug":
        zerolog.SetGlobalLevel(zerolog.DebugLevel)
    case "warn":
        zerolog.SetGlobalLevel(zerolog.WarnLevel)
    case "error":
        zerolog.SetGlobalLevel(zerolog.ErrorLevel)
    default:
        zerolog.SetGlobalLevel(zerolog.InfoLevel)
    }
}
```

Logic trùng với phần switch trong `init()`. Có thể extract để `init()` gọi lại `SetLevel()`, giảm duplication.

---

### Component 2: `cmd/othela/main.go` — Migration trọng tâm

#### [MODIFY] [`main.go`](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/main.go)

**a) Đổi import**: `"log"` → `"helvilette/pkg/log"`

**b) Connect `--log-level` flag vào zerolog**:

Hiện tại `--log-level` chỉ set `cfg.DebugMode = logLevel == "debug"`. Sau fix:

```go
// Đầu hàm Run, trước mọi log call:
log.SetLevel(logLevel)
```

**c) Xóa `DebugMode` khỏi `ServerConfig`** — zerolog xử lý level filtering.

**d) Migrate 12 call sites** — ví dụ before/after:

| # | Before (plain text) | After (structured JSON) |
|---|---|---|
| L53 | `log.Fatalf("[FATAL] --fleet-repo is required")` | `log.Fatal().Str("flag", "--fleet-repo").Msg("required flag not set")` |
| L56 | `log.Printf("Starting Helvilette Othela with LogLevel: %s, FleetRepo: %s, StateDir: %s, Port: %d", ...)` | `log.Info().Str("log_level", logLevel).Str("fleet_repo", fleetRepo).Str("state_dir", stateDir).Int("port", port).Msg("starting othela")` |
| L74-75 | `log.Printf("[WARN] Could not initialize SQLite at %s: %v", dbPath, err)` + `log.Printf("[WARN] Falling back to in-memory storage")` | `log.Warn().Err(err).Str("db_path", dbPath).Msg("could not initialize SQLite, falling back to in-memory storage — check directory exists and is writable, or use --state-dir")` |
| L77 | `log.Printf("[STORAGE] SQLite initialized at %s", dbPath)` | `log.Info().Str("db_path", dbPath).Msg("sqlite initialized")` |
| L95 | `log.Printf("Helvilette Othela is listening on %s...", addr)` | `log.Info().Str("addr", addr).Msg("othela listening")` |
| L102 | `log.Printf("[SHUTDOWN] Received signal: %v", sig)` | `log.Info().Str("signal", sig.String()).Msg("received shutdown signal")` |
| L104 | `log.Fatalf("Server failed unexpectedly: %v", err)` | `log.Fatal().Err(err).Msg("server failed unexpectedly")` |
| L116 | `log.Fatalf("[SHUTDOWN] Graceful shutdown failed: %v", err)` | `log.Fatal().Err(err).Msg("graceful shutdown failed")` |
| L122 | `log.Printf("[SHUTDOWN] Error closing resource: %v", err)` | `log.Error().Err(err).Msg("failed to close resource during shutdown")` |
| L126 | `log.Printf("[SHUTDOWN] Othela stopped gracefully.")` | `log.Info().Msg("othela stopped gracefully")` |

Tất cả sử dụng `logger := log.WithComponent("othela")` khai báo đầu hàm `Run`.

---

### Component 3: `cmd/othela/server.go` — Migration + xóa debugMode

#### [MODIFY] [`server.go`](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/server.go)

**a) Đổi import**: `"log"` → `"helvilette/pkg/log"`. Bỏ import `"fmt"` nếu không còn chỗ nào dùng (giữ lại nếu `fmt.Fprintf(w, ...)` vẫn cần cho HTTP response).

**b) Xóa `debugMode` khỏi `Server` struct** (L36) và `ServerConfig` (L93):

```diff
 type Server struct {
     router      *mux.Router
     loader      *playbook.Loader
     currentJob  Job
     playbooks   []playbook.Playbook
     nodeStore   storage.NodeStore
     reportStore storage.ReportStore
     mu          sync.RWMutex
     ready       atomic.Bool
-    debugMode   bool
 }
```

```diff
 type ServerConfig struct {
     NodeStore   storage.NodeStore
     ReportStore storage.ReportStore
     Loader      *playbook.Loader
-    DebugMode   bool
 }
```

**c) Xóa `SetDebug()` method** (L180-183).

**d) Xóa `s.debugMode` assignment** trong `NewServerWithConfig` (L113).

**e) Xóa tất cả `if s.debugMode { ... }` guards** — zerolog tự filter theo global level. `log.Debug().Msg(...)` sẽ bị suppressed khi level >= info.

**f) Migrate 17 call sites** — ví dụ trọng tâm:

| # | Before | After |
|---|---|---|
| L199 | `log.Printf("[REGISTER] Node %s registered with labels %v", ...)` | `logger.Info().Str("node_id", req.NodeID).Any("labels", req.Labels).Msg("node registered")` |
| L211-213 | `if s.debugMode { log.Printf("[DEBUG] [SYNC] Node %s is asking...") }` | `logger.Debug().Str("node_id", nodeID).Msg("node polling for work")` |
| L279-281 | `if s.debugMode { log.Printf("[DEBUG] Node %s has labels...") }` | `logger.Debug().Str("node_id", nodeID).Any("labels", labels).Msg("no matching node selectors")` |
| L328 | `log.Printf("[ERROR] Failed to save report: %v", err)` | `logger.Error().Err(err).Str("node_id", report.NodeID).Str("job_id", report.JobID).Msg("failed to save report — check storage backend connectivity")` |
| L338-342 | 5 dòng `log.Printf` với separator `---` | Một dòng structured: `logger.Info().Str("node_id", report.NodeID).Str("job_id", report.JobID).Str("status", report.Status).RawJSON("task_logs", report.TaskLogs).Msg("report received")` |
| L350 | `log.Printf("Othela Control Plane is listening on %s...", addr)` | **Xóa** — duplicate với `main.go` đã có |
| L402 | `log.Printf("[PLAYBOOKS] Returning %d playbooks", ...)` | `logger.Debug().Int("count", len(playbooks)).Msg("returning playbooks")` — đổi từ info xuống debug vì chạy mỗi request |
| L413 | `log.Printf("[ERROR] Failed to sync fleet repository %s: %v", ...)` | `logger.Error().Err(err).Str("repo", repo).Msg("failed to sync fleet repository — check repo URL accessibility and credentials")` |
| L433-434 | `if s.debugMode { log.Printf("[DEBUG] Fleet sync complete...") }` | `logger.Debug().Int("playbook_count", len(playbooks)).Msg("fleet sync complete")` |

Các handler method sử dụng `logger := log.WithComponent("othela")` tạo ở cấp package hoặc trong constructor.

> [!IMPORTANT]
> `ListenAndServe()` (L349-351) vẫn giữ — method này vẫn cần cho backward compat — nhưng xóa dòng `log.Printf` bên trong vì `main.go` đã log trước khi gọi.

---

### Component 4: Makefile — Dev mode cho Othela

#### [MODIFY] [`Makefile`](file:///home/stella/workspace/naughtian-helvilette/Makefile)

Hiện tại `run-agent` có `HELVILETTE_DEV=1` nhưng `run-othela` thì không. Thêm:

```makefile
# Run Othela (dev mode with human-readable logs)
run-othela:
	HELVILETTE_DEV=1 go run ./cmd/othela

# Run Othela (production mode with JSON logs)
run-othela-prod:
	go run ./cmd/othela
```

---

## Open Questions

> [!IMPORTANT]
> **Logger scope trong `server.go`**: Tạo `logger` ở đâu?
> - **Option A**: Package-level `var logger = log.WithComponent("othela")` — đơn giản, cả file dùng chung.
> - **Option B**: Mỗi handler tạo logger riêng với context (`node_id`, `job_id`) — giàu context hơn, nhưng verbose.
> - **Đề xuất**: Option A cho package-level, rồi trong handler nào cần thêm context thì derive: `reqLogger := logger.With().Str("node_id", nodeID).Logger()`. Giống pattern của `pkg/playbook/loader.go`.

> [!NOTE]
> **Report task_logs**: Hiện tại dump JSON blob qua `log.Printf("[REPORT] Full Output (JSON):\n%s", ...)`. Chuyển sang `RawJSON("task_logs", report.TaskLogs)` sẽ embed toàn bộ blob vào một JSON line. Nếu output lớn (>100KB), nên cắt hoặc chỉ log digest (status + task count). Ý kiến của bạn?

---

## Verification Plan

### Automated Tests

```bash
# Unit tests — đảm bảo không break existing logic
make test

# Build check — đảm bảo compile clean sau khi xóa stdlib "log" import
make build

# Format check
make fmt-check
```

### Manual Verification

1. `make run-othela` — verify output là human-readable (ConsoleWriter) khi `HELVILETTE_DEV=1`
2. `make run-othela-prod` — verify output là structured JSON, mỗi line parseable bởi `jq`
3. Verify `--log-level=debug` hiển thị debug messages, `--log-level=error` chỉ hiển thị error
4. Verify không còn `log.Printf` hay `"log"` import trong `cmd/othela/`

```bash
# Grep kiểm tra: không còn stdlib log import trong cmd/othela/
grep -rn '"log"' cmd/othela/
# Expected: no results

# Verify JSON output parseable
go run ./cmd/othela --fleet-repo=test 2>&1 | head -5 | jq .
```
