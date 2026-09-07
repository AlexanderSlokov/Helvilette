# Tiền Nhân Đã Nghĩ Gì Khi Chọn Log Library?

## Bản Đồ Lựa Chọn

```
Timeline:  2014 ─────── 2018 ─────── 2021 ─────── 2024+ ──────

K8s:       glog ──fork──► klog ──KEP-1602──► klog + logr interface
                   │                              │
                   └─ "glog unmaintained,          └─ "Đừng couple với
                      init() conflicts"               implementation"

k3s:       logrus ────────────────────────► logrus (maintenance mode)
                                                │
                                                └─ inherited from Rancher,
                                                   "works, don't touch"

Portainer: logrus ──────────────► zerolog
                                    │
                                    └─ "modernize, zero-alloc perf"

Go stdlib:                          log/slog (Go 1.21, 2023)
                                      │
                                      └─ "standard interface,
                                          swap backends freely"
```

---

## Kubernetes: "Fork rồi trừu tượng hóa"

**Chọn**: `glog` → `klog` → `klog` + `logr` interface

**Tại sao fork glog thành klog?**
- `glog` của Google bị **abandoned** — không ai maintain
- `glog` gọi `flag.Parse()` trong `init()` — phá vỡ mọi thứ trong containerized environment
- Không test được — `glog` write thẳng ra global state

**Tại sao không chuyển sang zap/zerolog?**
- Kubernetes có **hàng triệu dòng code** dùng `klog`. Thay thế = tự sát
- Thay vào đó, họ tạo **KEP-1602**: thêm structured logging (`InfoS`, `ErrorS`) lên trên `klog`
- Rồi đặt `logr` interface phía trước — code gọi `logr`, `logr` delegate xuống `klog`

**Triết lý**: *"Đừng couple với implementation. Couple với interface. Khi codebase đủ lớn, bạn không thể thay log library — bạn chỉ có thể thêm một lớp abstraction."*

> [!NOTE]
> K8s **không** chọn log library tốt nhất. Họ chọn **log interface** tốt nhất (`logr`), rồi giữ nguyên backend cũ. Đây là bài học cho mọi dự án lớn.

---

## k3s: "Thừa kế và sống chung"

**Chọn**: `logrus` (từ Rancher) + `klog` (từ K8s components)

**Tại sao logrus?**
- k3s sinh ra từ Rancher ecosystem, nơi `logrus` là standard
- k3s chạy **nhiều component trong một process** (API server, kubelet, containerd, etcd) — mỗi component dùng log library riêng
- Kết quả: output log của k3s là **mixed format** — y hệt vấn đề của Helvilette Othela

**Tại sao không migrate?**
- `logrus` đã vào **maintenance mode** (không nhận feature mới)
- Nhưng k3s không có lý do đủ mạnh để migrate — nó hoạt động, và log collection (Fluent Bit, Promtail) xử lý phía sau

**Triết lý**: *"Log library là infrastructure debt. Nếu nó không gây pain đủ lớn, đừng đụng vào. Để log aggregator xử lý format."*

---

## Portainer: "Modernize chủ động"

**Chọn**: `logrus` → **`zerolog`**

**Tại sao migrate sang zerolog?**
- Portainer là **single-binary web app** (giống Helvilette!) — không có vấn đề multi-component như k3s
- `logrus` vào maintenance mode, không có tương lai
- `zerolog` cho **zero-allocation**, API fluent, và JSON output mặc định
- Codebase đủ nhỏ để migration khả thi

**Triết lý**: *"Khi codebase còn nhỏ, hãy chọn đúng từ đầu. Chi phí migration tăng theo thời gian — làm sớm thì rẻ."*

> [!IMPORTANT]
> Portainer là case study gần nhất với Helvilette: single Go binary, web-based control plane, quản lý infrastructure. Họ đã chọn **zerolog** — cùng library mà `pkg/log` của Helvilette đang dùng.

---

## Và Bây Giờ Có `log/slog` (Go 1.21+)

Go 1.21 (2023) giới thiệu `log/slog` — structured logging trong stdlib:

| | zerolog | log/slog |
|--|---------|----------|
| Dependency | Third-party (`github.com/rs/zerolog`) | **Zero** — stdlib |
| Performance | Cực nhanh (zero-alloc) | Rất nhanh (nhưng thua zerolog ~20-30%) |
| API | Fluent chaining: `log.Info().Str("k","v").Msg("...")` | Traditional: `slog.Info("...", "k", "v")` |
| Backend swap | Phải wrap thủ công | Built-in `Handler` interface |
| Community trend | Mature, stable | **Hướng đi mới** của Go ecosystem |

Helvilette đang dùng **Go 1.25.6** — `slog` available.

---

## Vote Của Tôi Cho Helvilette

**Giữ zerolog. Chưa cần slog.**

Lý do:

1. **Helvilette đã invest vào zerolog** — `pkg/log/` wrapper tồn tại, Agent + `pkg/` packages đã dùng đúng. Vấn đề không phải zerolog sai, mà là Othela **chưa dùng nó**.

2. **Portainer precedent** — dự án gần nhất về kiến trúc (single Go binary, control plane) đã chọn chính xác zerolog.

3. **Codebase nhỏ** — ~30 call sites cần migrate trong `cmd/othela/`. Đây là 1-2 giờ làm việc, không phải bài toán kiến trúc.

4. **`pkg/log/log.go` đã là thin wrapper** — nó wrap zerolog với `WithComponent()`, `WithNodeID()`. Nếu tương lai cần chuyển sang `slog`, chỉ cần thay implementation bên trong wrapper này. Code gọi `log.Info().Msg(...)` không cần thay đổi gì.

5. **slog chưa cần thiết** — Helvilette không có vấn đề performance với zerolog, không có nhu cầu swap backend, và zerolog API (fluent chaining) phù hợp với style hiện tại hơn slog.

```
Helvilette hiện tại:

  cmd/agent/     ──► pkg/log (zerolog) ──► JSON     ✓ Đúng
  pkg/playbook/  ──► pkg/log (zerolog) ──► JSON     ✓ Đúng  
  pkg/systemd/   ──► pkg/log (zerolog) ──► JSON     ✓ Đúng
  cmd/othela/    ──► stdlib "log"      ──► PLAIN    ✗ SAI ← fix chỗ này

Sau fix:

  cmd/othela/    ──► pkg/log (zerolog) ──► JSON     ✓ Thống nhất
```

> [!TIP]
> Bài học từ K8s: `pkg/log/log.go` đang đóng vai trò giống `logr` — một thin wrapper decouple caller khỏi implementation. Giữ nó mỏng, đừng thêm business logic vào. Khi nào cần chuyển sang `slog`, chỉ cần thay phần bên trong wrapper.
