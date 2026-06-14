# Helvilette Project Takeaway

> **Mục đích:** Tài liệu chuyển giao thông tin dự án sang repo/conversation khác cho Frontend development.

---

## 1. Tổng quan Dự án

**Helvilette** (h8s) - OS Service Orchestration Framework cho Unix-like systems.

| Component | Mô tả | Tech |
|-----------|-------|------|
| **Othela** | Control Plane | Go, gorilla/mux |
| **Agent** | Node Agent | Go, go-systemd |
| **Dashboard** | Web/Desktop UI | Wails + Vue/Svelte |

**Kiến trúc:** Pull-based (như K8s kubelet)
- Agent outbound-only connection → firewall-safe
- Ansible làm execution engine
- Systemd làm service runtime

---

## 2. Backend API (Othela)

### REST Endpoints
```
GET  /api/v1/sync/{node_id}   → Trả về Job (playbook)
POST /api/v1/report           → Nhận execution report
```

### WebSocket (Planned)
```
/ws/logs/{node_id}   → Stream logs realtime
/ws/events           → Node status changes
```

### Data Types
```go
type Job struct {
    JobID           string `json:"job_id"`
    PlaybookContent string `json:"playbook_content"`
}

type Report struct {
    NodeID   string          `json:"node_id"`
    JobID    string          `json:"job_id"`
    Status   string          `json:"status"`
    TaskLogs json.RawMessage `json:"task_log"`
}
```

---

## 3. Frontend Stack Decisions

| Aspect | Decision |
|--------|----------|
| **Framework** | Wails v2 (Go backend + Web frontend) |
| **Frontend** | Vue.js hoặc Svelte |
| **CSS** | TailwindCSS + DaisyUI |
| **Icons** | Lucide hoặc Heroicons |
| **Charts** | Chart.js hoặc Apache ECharts |

**Tại sao Wails?**
- Chia sẻ Go code với Othela
- Desktop native + Web trong cùng codebase
- Bundle nhẹ (~10MB vs Electron 100MB+)

---

## 4. Design Style: Death Stranding-inspired

### Phong cách chính
| Style | Áp dụng |
|-------|---------|
| **Industrial Brutalism** | Đường nét cứng cáp, góc cạnh, công năng tối đa |
| **High-Tech Minimalist** | Màu trung tính + accent neon |
| **Diegetic UI** | Thông tin như thiết bị thực trong thế giới |
| **Dieter Rams** | Less is more, chức năng rõ ràng |

### Typography
- **Headings:** Sans-serif góc cạnh (SST Roman style)
- **Data/Numbers:** Monospace (JetBrains Mono, Fira Code)
- **Body:** Inter, Roboto

### Color Palette
```css
--bg-primary: #0a0a0f;      /* Đen xanh đậm */
--bg-secondary: #12141a;    /* Panel background */
--accent-cyan: #00d4ff;     /* Highlight chính */
--accent-orange: #ff6b35;   /* Warning/Alert */
--accent-green: #00ff6a;    /* Success */
--accent-red: #ff3366;      /* Error */
--text-primary: #e0e0e0;    /* Text chính */
--text-muted: #6b7280;      /* Text phụ */
```

### UI Effects (subtle, không nặng)
- Glassmorphism nhẹ (`backdrop-filter: blur(8px)`)
- Border glow on hover
- Subtle scan lines (optional)
- Monospace number transitions

### Reference Image
![Death Stranding UI Reference](uploaded_media_1769856472514.png)

**Đặc điểm từ hình:**
- List view với grouped items (categories)
- Right panel: Detail view của item selected
- Progress bars với color coding
- Status badges (S, M với colors)
- Dense information layout
- Dark theme với cyan/orange accents

---

## 5. Dashboard Screens (Planned)

### 5.1. Dashboard Overview
- Node count summary (active/inactive/failed)
- Recent jobs list
- System health metrics

### 5.2. Node List
- Table với status, last seen, current job
- Filter/search
- Click để xem detail

### 5.3. Node Detail
- Node info (hostname, IP, profile)
- Current/recent jobs
- Systemd unit states
- Log stream

### 5.4. Job History
- List jobs với status badges
- Filter by node, status, date
- Job detail với Ansible output

---

## 6. Repository Structure (Frontend)

```
helvilette-ui/
├── wails.json
├── main.go              # Wails entry
├── frontend/
│   ├── src/
│   │   ├── App.vue
│   │   ├── components/
│   │   │   ├── NodeList.vue
│   │   │   ├── JobCard.vue
│   │   │   └── LogStream.vue
│   │   ├── views/
│   │   │   ├── Dashboard.vue
│   │   │   ├── Nodes.vue
│   │   │   └── Jobs.vue
│   │   └── styles/
│   │       └── death-stranding.css
│   ├── tailwind.config.js
│   └── package.json
└── README.md
```

---

## 7. Links & Resources

- **Backend Repo:** `/mnt/e/Helvilette`
- **Design Reference:** Death Stranding Terminal UI
- **Wails Docs:** https://wails.io/docs/introduction
- **DaisyUI:** https://daisyui.com/

---

*Tạo ngày: 2025-01-31*
