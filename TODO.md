# Helvilette Future Roadmap & TODOs

## 0. Architecture Decisions (Đã chốt)

### 0.1. Hybrid Model cho `helvilette.yml`
**Quyết định:** `helvilette.yml` nằm **TRONG** Ansible Playbook repo (như `Chart.yaml` của Helm)

**Lý do:**
- `helvilette.yml` chứa **metadata + defaults** (version, description, profiles, default vars)
- **Secrets + environment-specific values** được Othela inject tại runtime (không commit vào Git)
- GitOps-friendly: Một commit = một phiên bản hoàn chỉnh
- Secrets management: Othela giữ secrets trong SQLite/etcd, hoặc tích hợp Vault/SOPS

**Cấu trúc mẫu:**
```
playbooks-repo/
├── nginx-stack/
│   ├── helvilette.yml      # Metadata + defaults
│   ├── requirements.yml    # Ansible Galaxy deps
│   ├── playbook.yml
│   └── roles/
```

### 0.2. Logging Library: `zerolog`
**Quyết định:** Sử dụng `github.com/rs/zerolog` cho structured logging

**So sánh:**
| Library | Performance | Dependencies | API |
|---------|-------------|--------------|-----|
| zerolog ✅ | Zero-allocation | External | Chainable |
| slog | Good | Stdlib | Verbose |
| zap | Excellent | External | Dual-mode |

**Lý do chọn zerolog:**
- Zero-allocation JSON logging (critical cho high-frequency systemd events)
- Chainable API: `log.Info().Str("unit", name).Msg("started")`
- Battle-tested trong production

### 0.3. Frontend Stack (2025-01-31)
**Quyết định:**
- **Framework:** Wails v2 + Go backend
- **Frontend:** Vue.js hoặc Svelte
- **CSS:** TailwindCSS + DaisyUI (dark theme, component library)
- **API Pattern:** REST cho CRUD, WebSocket cho streaming (logs, events)

**Lý do chọn Wails:**
- Chia sẻ code Go với Othela (import `pkg/` types)
- Bundle nhẹ hơn Electron (~10MB vs 100MB+)
- Native window, không phải Chromium bloat

**Design Style:** Death Stranding-inspired
- Industrial Brutalism + High-Tech Minimalist
- Diegetic UI (thông tin như thiết bị thực)
- Dense UI với monospace fonts cho data
- Dark theme với accent xanh/cam

---

## 1. Quản lý Ansible Playbook (The Core Engine)
Hiện tại đang hardcode string trong Go. Cần chuyển sang cơ chế quản lý file thực thụ.

### Yêu cầu:
- **GitOps-driven:** Othela phải biết tự pull Playbooks từ một Git Repository (GitHub/GitLab) về `helvillette/othela/data/playbooks`.
- **Ansible Galaxy Support:** Tự động phát hiện file `helvillette.yml` trong repo để cài đặt các Roles/Collections cần thiết (`ansible-galaxy install -r ...`).

### Đề xuất giải pháp (Technical Proposal):
1.  **Repo Watcher (Go Routine):** Một thread chạy ngầm trên Othela, định kỳ `git pull` từ remote repo.
2.  **Versioning:** Mỗi lần commit mới sẽ có hash SHA. Othela dùng SHA này làm "Job Version" để đảm bảo các Agent update lên phiên bản mới nhất.
3.  **Local Cache:** Othela cache playbook ra đĩa. Khi Agent hỏi, Othela đọc file từ đĩa gửi đi (hoặc nén trả về URL download nếu file lớn).

## 2. Agent Intelligence (State Awareness)
- **Drift Detection:** Thay vì chạy đè (`force`), Agent nên chạy `check_mode` (-C) trước. Nếu kết quả là `changed=0`, báo "Green". Nếu `changed>0`, mới chạy thật (hoặc báo vàng chờ duyệt).
- **Security:** Triển khai mTLS cho kết nối gRPC giữa Agent và Othela.

## 3. UI/Dashboard
### 3.1. Core Features:
- Danh sách Node với status badges
- Trạng thái Job gần nhất
- Log realtime (Stream qua WebSocket)

### 3.2. Design Reference:
- **Style:** Death Stranding Terminal UI
- **Fonts:** SST Roman, Sackers Gothic, Monospace cho data
- **Colors:** Dark base + neon accents (cyan, orange)
- **Effects:** Subtle glitch, hologram glow (không quá nặng)
