# Kế hoạch Triển khai: Helvilette Walking Skeleton (PoC)

Đây là bản kế hoạch chi tiết để xây dựng phiên bản "Walking Skeleton" của Helvilette nhằm kiểm chứng kiến trúc theo yêu cầu của bạn.

Mục tiêu chính: Xác nhận khả năng **Go binary** có thể hoạt động như một wrapper tin cậy cho **Ansible**, giao tiếp với Control Plane theo mô hình **Outbound-only**.

## User Review Required
> [!IMPORTANT]
> **Yêu cầu môi trường:** Cần đảm bảo môi trường development đã cài đặt sẵn `Ansible` để Agent có thể thực thi lệnh `ansible-playbook`.
> **Giả định:** PoC này sẽ chạy local (cả Server và Agent trên cùng một máy hoặc trong mạng LAN), Agent sẽ target chính `localhost` để an toàn.

## Proposed Changes

### 1. Cấu trúc Dự án (Project Structure)
Sử dụng cấu trúc Go tiêu chuẩn đơn giản cho PoC:
```text
helvilette/
├── go.mod
├── README.md
├── cmd/
│   ├── othela/      # Source code cho Control Plane
│   │   └── main.go
│   └── agent/       # Source code cho Agent
│       └── main.go
```

### 2. Thành phần A: Othela (Control Plane)
**Trách nhiệm:** Giả lập vai trò của "bộ não" trung tâm, lưu trữ Desired State giả lập và hứng kết quả báo cáo.

#### [NEW] [cmd/othela/main.go](file:///e:/Helvilette/cmd/othela/main.go)
-   **HTTP Server:** Lắng nghe tại port `:8080`.
-   **In-Memory Store:** Sử dụng `map` hoặc `struct` đơn giản để lưu 1 Mock Job cố định. **Mock Job** này sẽ là một playbook đơn giản in ra "Hello Wunjo!".
-   **API Endpoints:**
    -   `GET /api/v1/sync/{node_id}`:
        -   Mô phỏng Agent hỏi: *"Có việc gì cho em làm không?"*
        -   Trả về JSON: `{ "job_id": "job-123", "playbook": "...yaml content..." }`
    -   `POST /api/v1/report`:
        -   Nhận kết quả từ Agent sau khi chạy xong.
        -   Log toàn bộ JSON body receive được ra `STDOUT` để debug.

### 3. Thành phần B: Helvilette Agent (Client)
**Trách nhiệm:** Worker node, kéo việc về và sai bảo Ansible làm việc, sau đó báo cáo kết quả.

#### [NEW] [cmd/agent/main.go](file:///e:/Helvilette/cmd/agent/main.go)
-   **Polling Loop:** `time.Ticker` 5 giây một lần gọi lên Othela.
-   **Job Processing:**
    1.  Gọi `GET /api/v1/sync/agent-01`.
    2.  Check `job_id`. Nếu mới (so với job cũ trong RAM) -> Xử lý.
    3.  Ghi nội dung playbook vào `/tmp/helvilette_job_<id>.yml`.
-   **Execution Engine (Core Logic):**
    -   Sử dụng `exec.Command` để gọi:
        ```bash
        ansible-playbook -i "localhost," -c local /tmp/helvilette_job_<id>.yml
        ```
    -   **Environment Variable:** Inject `ANSIBLE_STDOUT_CALLBACK=json` vào process environment. Đây là chìa khóa để parse output dễ dàng.
-   **Reporting:**
    -   Capture `Stdout` (đã là JSON nhờ callback).
    -   Unmarshal sơ bộ để đảm bảo valid JSON (sanity check).
    -   Gửi nguyên cục JSON đó về `POST /api/v1/report` của Othela.

### 4. Tài liệu & Đánh giá (Sanity Check)

#### [NEW] [README.md](file:///e:/Helvilette/README.md)
-   Hướng dẫn Build & Run 2 binaries.
-   **Phân tích Kiến trúc:** Trả lời 2 câu hỏi cốt lõi của bạn:
    1.  Độ tin cậy của `ANSIBLE_STDOUT_CALLBACK=json` cho production logging/streaming.
    2.  Rủi ro của mô hình Pull-based (Agent tự kéo config) khi mất kết nối Control Plane.

### 5. [FUTURE] Component C: Playbook Manager Module (Othela)
**Mục tiêu:** Quản lý Playbook tập trung, hỗ trợ GitOps và Ansible Galaxy.

-   **Git Watcher Service:**
    -   Tích hợp thư viện `go-git`.
    -   Logic: Clone/Pull repo chứa playbook định kỳ (ví dụ 1 phúc/lần).
    -   Phát hiện thay đổi (dựa trên Commit Hash).

-   **Galaxy Resolver:**
    -   Quét thư mục repo, tìm `requirements.yml`.
    -   Tự động chạy `ansible-galaxy install` vào thư mục `roles/` của Othela.

-   **Dispatcher Logic nâng cấp:**
    -   Thay vì gửi raw string, Othela sẽ gửi một **Bundle** (URL download file .zip chứa cả playbook + roles) hoặc đọc file từ disk.
    -   Agent sẽ download, unzip và chạy.

## Verification Plan

### Automated Tests
-   Không viết Unit Test cho PoC này để tối ưu tốc độ.
-   Logic được kiểm chứng bằng Integration Test thủ công (E2E).

### Manual Verification
1.  **Dựng Server:** Chạy `go run ./cmd/othela`.
2.  **Dựng Agent:** Chạy `go run ./cmd/agent` ở một terminal khác.
3.  **Quan sát Othela Console:**
    -   Thấy request polling từ Agent.
    -   Thấy Agent nhận Job.
    -   **Quan trọng:** Thấy JSON Report in ra màn hình Othela báo cáo Ansible chạy thành công (Task "debug" in ra "Hello Wunjo!").
