# Helvilette — Pivot Statement

## Trước cuộc trò chuyện

> "Helvilette là một OS service orchestration framework dùng Ansible Playbook"
> — Mơ hồ, chung chung, lạc trong biển đỏ orchestrator

## Sau cuộc trò chuyện

> **Helvilette là pull-based delivery layer cho Ansible — biến playbook thành hệ thống tự vận hành, không SSH, không CI/CD pipeline, không phụ thuộc vào bất kỳ ai nhấn Enter.**

---

## Pivot từ SAI sang ĐÚNG

| Trước (sai) | Sau (đúng) |
|---|---|
| "Orchestrator framework" | **Ansible delivery + drift protection layer** |
| Cạnh tranh với K8s, Salt, Puppet | **Ngồi DƯỚI tất cả, ở tầng OS/systemd** |
| "Lightweight alternative to AWX" | **Missing piece biến Ansible từ tool → system** |
| Target: mọi người | **Target: 12-50 VM, hybrid infra, team 1-2 người** |
| Value: "chạy playbook" | **Value: loại bỏ SSH key, loại bỏ bus factor, chống config drift** |

---

## 5 Trụ Cột Giá Trị

### 1. 🔑 Tiêu diệt `~/.ssh`
Push-based Ansible buộc root key tập trung trên laptop một người. Helvilette đảo ngược: agent pull từ bên trong server ra ngoài. Không inbound port. Không SSH key. Laptop mất? Kệ.

### 2. 🔄 Reconciliation loop cho thế giới không có K8s
K8s có controller loop tự heal. 80% infra còn lại thì không. Helvilette mang mô hình desired-state reconciliation xuống tầng OS/systemd — nơi mà K8s không với tới.

### 3. 🐔 Phá vòng lặp con gà quả trứng
Ansible cài Helvilette (lần SSH cuối cùng). Helvilette delivery mọi Ansible playbook sau đó. Con gà đẻ ra máy đẻ trứng tự động, rồi con gà về hưu.

### 4. 🐈‍⬛ Quản lý cả người quản lý
Helvilette sống ở tầng systemd — dưới K8s, dưới Docker, dưới mọi orchestrator. Nó có thể rolling update kubelet, restart kube-apiserver, heal cái mà K8s không thể tự heal — vì K8s không thể mổ não chính mình.

### 5. 🚌 Xóa bus factor
Kiến thức infra sống trong Git repo + Helvilette agent, không phải trong đầu "thằng Tuấn". Tuấn nghỉ? Hệ thống vẫn tự chạy. Người mới vào chỉ cần git push.

---

## Khách Hàng Mục Tiêu

```
Startup / SMB Việt Nam (và toàn cầu)
├── 12-50 VMs
├── Hybrid: Proxmox + bare-metal + VPS (Mắt Bão, BKNS, Viettel IDC...)
├── Team infra: 1-2 người
├── Đã có Ansible playbooks
├── Đã ghét CI/CD pipeline glue
├── Không đủ scale cho K8s
└── Đang giữ root SSH key trên laptop cá nhân
```

## Helvilette KHÔNG PHẢI

- ❌ Không phải orchestrator — không cạnh tranh với K8s/Nomad/Salt
- ❌ Không phải CI/CD — không cạnh tranh với GitHub Actions/GitLab CI
- ❌ Không phải Ansible replacement — Ansible vẫn là engine
- ❌ Không phải configuration management tool — Ansible làm việc đó

## Helvilette LÀ

- ✅ Delivery layer: đưa playbook từ Git → server mà không cần SSH
- ✅ Reconciliation engine: đảm bảo server luôn ở desired state
- ✅ Infrastructure immune system: heal cái mà orchestrator không tự heal được
- ✅ Bus factor eliminator: hệ thống tự vận hành, không phụ thuộc con người

---

## Tagline Candidates

1. *"Dùng Ansible cài Helvilette một lần. Không bao giờ cần SSH cho Ansible nữa."*
2. *"Ansible là thợ sửa ống nước giỏi nhất thế giới. Helvilette là bảo vệ ngồi lại canh."*
3. *"K8s reconciliation loop, cho thế giới không có K8s."*
4. *"The last SSH you'll ever need."*

---

## Tiếp Theo: Code Gì?

Dựa trên pivot này, priority thay đổi hoàn toàn:

### Must-have (trước khi demo được)
1. **Node targeting** — Othela gán job cho node cụ thể (theo label/tag), không broadcast
2. **Persistence** — SQLite cho Othela (job history, node registry, reports)
3. **Basic auth** — API key hoặc pre-shared token cho agent ↔ Othela
4. **Drift detection** — Agent chạy `ansible-playbook --check` theo schedule, report drift

### Nice-to-have (sau khi core ổn)
5. Dashboard UI (Othela web)
6. Multi-repo support
7. Scheduled playbook runs
8. Webhook trigger (git push → Othela notify agents)

### Drop / Deprioritize
- ~~CNCF application~~ (quá sớm)
- ~~Death Stranding UI~~ (fun nhưng không cần bây giờ)
- ~~Wails desktop app~~ (web UI đủ rồi)
