# Issue #31: Othela Inconsistent Log Output — Audit Report

## Issue Summary

> **Title**: Othela: Inconsistent log output (mix of JSON and plain text)
> **State**: OPEN

Khi Othela khởi động, output log bị pha trộn giữa plain text (`log.Printf`) và structured JSON (`zerolog`). Ví dụ:
```
2026/08/30 15:43:39 Starting Helvilette Othela with LogLevel: info, PlaybookDir: /var/lib/helvilette/playbooks...
{"level":"info","component":"playbook-loader","count":0...}
```

---

## Mức Độ Nghiêm Trọng: Khá Tệ

Có **hai hệ thống log hoàn toàn khác nhau** đang chạy song song trong cùng một binary. Đây không phải là lỗi nhỏ — nó là vấn đề kiến trúc.

---

## Phân Tích Chi Tiết

### 1. Hai Hệ Thống Log Song Song

| Hệ thống | Package | Format | Ai dùng? |
|-----------|---------|--------|----------|
| `"log"` (stdlib) | Go stdlib | Plain text, prefix `2006/01/02 15:04:05` | **Othela** (`cmd/othela/`) |
| `"helvilette/pkg/log"` (zerolog) | [log.go](file:///home/stella/workspace/naughtian-helvilette/pkg/log/log.go) | Structured JSON | **Agent** (`cmd/agent/`), `pkg/playbook/`, `pkg/systemd/` |

### 2. File-by-File Breakdown

#### Thủ phạm chính: [`cmd/othela/server.go`](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/server.go) — **17 lần gọi `log.Printf`/`log.Fatalf`**

| Line | Call | Vấn đề |
|------|------|--------|
| L199 | `log.Printf("[REGISTER] Node %s registered...")` | Plain text, không có structured fields |
| L212 | `log.Printf("[DEBUG] [SYNC] Node %s is asking...")` | Debug level giả lập bằng `if s.debugMode` thay vì dùng log level |
| L280 | `log.Printf("[DEBUG] Node %s has labels %v...")` | Tương tự — fake debug level |
| L328 | `log.Printf("[ERROR] Failed to save report: %v")` | Error giả lập bằng prefix string `[ERROR]` |
| L334 | `log.Printf("[ERROR] Failed to update node status: %v")` | Tương tự |
| **L338-342** | **5 dòng `log.Printf` liên tiếp cho Report** | **Tệ nhất**: dùng `---` separator, dump JSON thô vào plain text log |
| L350 | `log.Printf("Othela Control Plane is listening...")` | Duplicate với dòng tương tự trong `main.go:95` |
| L402 | `log.Printf("[PLAYBOOKS] Returning %d playbooks")` | Nên là debug, đang chạy ở mọi request |
| L413-434 | 4 lần `log.Printf` trong `StartFleetSync` | Error/Debug giả lập |

#### Thủ phạm thứ hai: [`cmd/othela/main.go`](file:///home/stella/workspace/naughtian-helvilette/cmd/othela/main.go) — **12 lần gọi `log.Printf`/`log.Fatalf`**

| Line | Call | Vấn đề |
|------|------|--------|
| L53 | `log.Fatalf("[FATAL] --fleet-repo is required")` | Nên dùng zerolog Fatal |
| L56 | `log.Printf("Starting Helvilette Othela with LogLevel: %s...")` | Startup log bằng plain text |
| L74-77 | `log.Printf("[WARN]...")`/`log.Printf("[STORAGE]...")` | Fake log levels qua prefix |
| L95 | `log.Printf("Helvilette Othela is listening on %s...")` | Plain text |
| L102-126 | 6 lần trong shutdown sequence | Toàn bộ shutdown logging là plain text |

#### Đã đúng: `cmd/agent/main.go` — Dùng `helvilette/pkg/log` (zerolog)

```go
log.Fatal().Err(err).Msg("failed to load configuration")  // ✓ Structured
log.Info().Str("signal", sig.String()).Msg("received shutdown signal")  // ✓ Structured
```

#### Đã đúng: `pkg/playbook/loader.go`, `pkg/systemd/` — Dùng zerolog

```go
logger := log.WithComponent("playbook-loader")  // ✓ Component tagged
logger.Warn().Err(err).Str("path", path).Msg("failed to access path")  // ✓ Structured
```

---

## 3. Tổng Hợp Các Vấn Đề

### Vấn đề loại A: Sai hệ thống log (Critical)
> [!CAUTION]
> Toàn bộ `cmd/othela/` (2 file, **29 call sites**) dùng stdlib `"log"` thay vì `"helvilette/pkg/log"`. Output sẽ là plain text xen kẽ với JSON từ các package khác.

### Vấn đề loại B: Fake log levels (High)
Log level được giả lập bằng string prefix `[ERROR]`, `[WARN]`, `[DEBUG]` thay vì dùng zerolog level. Hậu quả:
- **Không filter được** — `[ERROR]` trong `log.Printf` vẫn xuất ra khi set level = `error`
- **Log aggregator không parse được** — Datadog/Loki/ELK không nhận diện level từ plain text prefix
- **Debug noise** — `if s.debugMode { log.Printf("[DEBUG]...") }` là manual reimplementation của log level filtering

### Vấn đề loại C: Log noise & bad practices (Medium)
- **Separator lines**: `log.Printf("---------------------------------------------------")` — noise trong structured logging
- **Duplicate messages**: "listening on..." xuất hiện ở cả `main.go:95` và `server.go:350`
- **`[PLAYBOOKS] Returning %d playbooks`**: chạy ở mỗi request, nên là debug level
- **Report dump**: `log.Printf("[REPORT] Full Output (JSON):\n%s", ...)` — dump JSON blob vào plain text log line, phá vỡ cả hai format

### Vấn đề loại D: `--log-level` flag bị bỏ qua (High)
> [!WARNING]
> `cmd/othela/main.go` có flag `--log-level` nhưng chỉ dùng để set `cfg.DebugMode = logLevel == "debug"`. Nó **không** gọi `zerolog.SetGlobalLevel()` và cũng **không ảnh hưởng gì** tới stdlib `log.Printf` — stdlib Go không có concept log level.
>
> `pkg/log/log.go` đọc level từ env `HELVILETTE_LOG_LEVEL`, nhưng Othela không set env này. Hai cơ chế cấu hình log level hoàn toàn tách rời.

---

## 4. So Sánh Agent vs Othela

| Tiêu chí | Agent (`cmd/agent/`) | Othela (`cmd/othela/`) |
|-----------|---------------------|----------------------|
| Log library | `helvilette/pkg/log` (zerolog) | stdlib `"log"` |
| Output format | Structured JSON | Plain text |
| Log levels | Zerolog levels (Debug/Info/Warn/Error/Fatal) | Fake via string prefix |
| Level filtering | Works via `zerolog.SetGlobalLevel` | `if s.debugMode` (only debug, no warn/error) |
| Component tagging | `log.WithComponent("agent")` | None |
| Machine-parseable | Yes | No |

> [!IMPORTANT]
> **Kết luận**: Othela đang ở trạng thái "prototype logging" — dùng stdlib print statements với manual prefix. Agent đã được migrate sang structured logging đúng cách. Toàn bộ `cmd/othela/` cần được migrate sang `helvilette/pkg/log`.
