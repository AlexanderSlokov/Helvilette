# Helvilette — First Light

**Bài test đầu tiên cho Othela + 1 Agent**
Mục tiêu: **không phải** để Helvilette chạy đúng. Mục tiêu là để nó chạy **một lần**, và anh có một danh sách việc cụ thể thay vì một nỗi sợ mơ hồ.

---

## 0. Luật của đêm nay

Đọc phần này trước, và tuân thủ. Nó quan trọng hơn phần kỹ thuật.

1. **Một node. Detection-only.** Không auto-apply, không opt-in gì hết. Đúng default của anh.
2. **Không sửa code trong lúc test.** Thấy bug → ghi vào logbook → chạy tiếp. Sửa vào ngày mai. Đây là luật khó giữ nhất và cũng là luật quan trọng nhất: nếu anh vừa chạy vừa sửa, cuối buổi anh sẽ không có dữ liệu, chỉ có mệt.
3. **Snapshot trước.** Cả hai VM. Tên snapshot: `pre-helvi`. Có snapshot rồi thì không có gì hỏng được vĩnh viễn.
4. **Timebox 90 phút.** Hết giờ là dừng, kể cả đang dở. Round nào chưa chạy thì ghi "not reached", không phải "fail".
5. **Copy output nguyên văn.** Xấu cỡ nào cũng giữ nguyên. Không dọn dẹp, không tóm tắt lại cho đỡ ngượng. Cái xấu chính là dữ liệu.
6. **Điểm số là để đo Helvilette, không phải đo anh.** Xem thang điểm ở mục 6 trước khi bắt đầu, để biết ngưỡng "bình thường" nó thấp cỡ nào.

---

## 1. Môi trường

### OS: Debian 12 (Bookworm)

Lý do chọn: nó chán. Đó là toàn bộ lý do.

- Ubuntu có snap, có unattended-upgrades hay tự chạy ngang, có `needrestart` nhảy vào giữa apt → tạo nhiễu không phải của Helvilette.
- RHEL-family có subscription-manager, SELinux enforcing → lần đầu mà dính SELinux thì anh sẽ không phân biệt được lỗi của ai.
- Debian 12: systemd chuẩn, Python 3.11 sẵn, apt im lặng. Nếu có gì hỏng, gần như chắc chắn là Helvilette hỏng.

**Nguyên tắc chung: mọi thứ ngoài Helvilette đều phải chán đến mức không thể là nghi phạm.**

### Topology

| Vai trò | Máy | Spec | Ghi chú |
|---|---|---|---|
| Othela (control plane) | VM trên Proxmox hoặc Vagrant (KVM) | 2 vCPU / 2GB / 20GB | Debian 12 minimal |
| Gitea | Docker trên **chính host Othela** | — | Xem lý do bên dưới |
| Agent node | VM riêng trên Proxmox hoặc Vagrant (KVM) | 1 vCPU / 1GB / 10GB | Debian 12 minimal, **fresh install** |

*(Ghi chú: Nếu dùng Vagrant, chạy `vagrant up` trong thư mục `vagrant/` để tự động spin up cả hai VM và cài đặt sẵn qua Ansible)*

### VM, không phải container

Đừng dùng LXC hay Docker container làm agent node.

- Helvilette agent gần như chắc chắn chạy dưới systemd. Trong container, systemd hoặc không có, hoặc chạy què.
- Playbook test có task về service (`systemctl`) và sysctl — cả hai đều hành xử khác trong container.
- Anh cần snapshot/rollback thật sự, và sau này cần test cắt điện giữa chừng. VM làm được, container thì lằng nhằng.

Container sẽ khiến anh debug môi trường thay vì debug Helvilette.

### Gitea: Docker Compose, trên host Othela

```yaml
services:
  gitea:
    image: gitea/gitea:1.22
    container_name: gitea
    restart: unless-stopped
    environment:
      - GITEA__database__DB_TYPE=sqlite3
      - GITEA__server__ROOT_URL=http://<othela-ip>:3000/
    volumes:
      - ./gitea-data:/data
    ports:
      - "3000:3000"
      - "2222:22"
```

SQLite, HTTP thuần, không TLS.

**Đặt chung host với Othela** — đây là shortcut của test rig, không phải kiến trúc. Lý do: bớt một máy, bớt một lớp mạng có thể hỏng. Ghi rõ trong logbook là "test rig shortcut" để sau này không ai (kể cả anh, sáu tháng nữa) tưởng đó là design.

Repo: `helvi-test/baseline`, branch `main`, một user duy nhất, token đọc.

---

## 2. Playbook: role `baseline-node`

Cố ý làm nhỏ, chán, và **idempotent**. Bảy task, mỗi task test một kiểu drift khác nhau.

```
roles/baseline-node/
├── tasks/main.yml
├── templates/chrony.conf.j2
├── handlers/main.yml
└── files/report-uptime.sh
```

**Các task và kiểu drift chúng phải bắt được:**

| # | Task | Kiểu drift kỳ vọng | Operator ký hiệu |
|---|---|---|---|
| 1 | Cài package `chrony` | Package bị gỡ | `+` |
| 2 | Template `/etc/chrony/chrony.conf` | Nội dung file đổi | `~` |
| 3 | Service `chrony` enabled + started | Service bị stop | `~` hoặc `↻` |
| 4 | User `helvi-test` tồn tại, shell `/usr/sbin/nologin` | User bị xoá | `+` |
| 5 | File `/etc/sysctl.d/99-helvi.conf` (`vm.swappiness=10`), mode `0640` | Giá trị đổi | `~` |
| 6 | Mode/owner của file trên: `0640 root:root` | `chmod 0777` | `~` |
| 7 | `command: /usr/local/bin/report-uptime.sh` — **cố ý không có `creates`, không có `changed_when`** | Không check-safe được | `?` |

Task số 7 là task quan trọng nhất trong bài test. Nó tồn tại chỉ để trả lời một câu hỏi: **Helvilette có trung thực về cái nó không biết không?**

---

## 3. Các round

### Round 0 — Baseline sạch

Chạy playbook bằng `ansible-playbook` tay một lần cho node khớp hoàn toàn. Rồi cho Helvilette chạy.

**Kỳ vọng:** báo "synchronized" hoặc tương đương. Zero drift (trừ task 7, phải hiện `?`).

Đây là bài test **false positive**. Một tool GitOps báo drift khi không có drift thì vô dụng hơn là không có tool — vì nó dạy operator bỏ qua báo cáo.

Ghi lại: có bao nhiêu dòng output cho một node hoàn toàn sạch? (OX của anh nói: phải rất ít.)

---

### Round 1 — Drift đã biết trước

Trên agent node, chạy đúng bốn lệnh này. Không hơn.

```bash
sed -i 's/vm.swappiness=10/vm.swappiness=60/' /etc/sysctl.d/99-helvi.conf
chmod 0777 /etc/sysctl.d/99-helvi.conf
userdel helvi-test
systemctl stop chrony
```

**Kỳ vọng:** đúng 4 drift. Không thừa, không thiếu, đúng ký hiệu.

Đây là bài test cốt lõi. Anh biết trước đáp án, nên anh chấm được chính xác.

Ghi lại: nó báo mấy cái? Đúng mấy? Có báo nhầm cái gì không?

---

### Round 2 — Sự trung thực về vùng mù

Không làm gì thêm. Chỉ nhìn kỹ task 7 trong output của Round 0 và 1.

**Kỳ vọng:** `?` hiện rõ ràng, kèm giải thích ngắn kiểu "task này không chạy được ở chế độ check, trạng thái thật không xác định".

**Fail nếu:** nó im lặng bỏ qua, hoặc tệ hơn — báo là "ok".

Đây là ranh giới đạo đức của tool. Một tool nói "tôi không biết" là tool dùng được. Một tool giả vờ biết là tool nguy hiểm.

---

### Round 3 — Dry-run gãy

Push một commit thêm task này vào cuối role:

```yaml
- name: Deliberately broken task
  template:
    src: does-not-exist.j2
    dest: /etc/helvi-broken.conf
```

**Kỳ vọng:**
- Agent dry-run, gãy.
- **Dừng lại.** Không chạy tiếp các task sau.
- Báo lên Othela với trạng thái phân biệt được: "dry-run failed" ≠ "drift detected" ≠ "node unreachable".
- Thông báo cho operator biết phải làm gì (SSH vào? xem log ở đâu?).

Đây là bài test quan trọng thứ hai. Cách một tool hỏng nói lên nhiều về nó hơn cách nó chạy đúng.

Sau round này: `git revert`, push lại.

---

### Round 4 — Vòng poll

Push một commit vô hại (sửa một dòng comment trong `chrony.conf.j2`).

**Bấm đồng hồ.** Bao lâu thì agent nhận ra?

**Kỳ vọng:** đúng bằng poll interval đã cấu hình, ±10%. Và operator phải nhìn được lần poll gần nhất là khi nào — không có cái đó thì "im lặng" và "chết" trông giống hệt nhau.

---

## 4. Logbook

Điền trong lúc chạy. Không đợi đến cuối.

```
Round: ___
Giờ: ___
Lệnh đã chạy:

Output (nguyên văn, dán vào đây):


Quan sát 1:
Quan sát 2:
Quan sát 3:

Phân loại (điền sau, KHÔNG điền lúc đang chạy):
  [ ] Bug thật
  [ ] Lỗi config của tôi
  [ ] Log xấu (đúng nhưng khó đọc)
  [ ] Chưa implement
```

Cột "Phân loại" là thứ biến một buổi tối tệ thành một backlog. Rất nhiều thứ trông như thảm hoạ lúc 11 giờ đêm hoá ra chỉ là "log xấu".

---

## 5. Điều kiện dừng

Dừng ngay, ghi lại, không cố cứu:

- Agent làm hỏng node dù đang ở detection-only → **dừng hẳn.** Đây là bug nghiêm trọng nhất có thể có. Ghi lại thật chi tiết.
- Othela không khởi động được sau 20 phút → dừng. Đó là bug ở bước enrollment, xứng đáng có một buổi riêng.
- Hết 90 phút.

---

## 6. Thang điểm

**Đọc mục "Cách hiểu điểm" ở cuối trước khi chấm.**

### A. Sống sót (10 điểm)

| | Điểm |
|---|---|
| Othela khởi động và giữ được process | 2 |
| Agent enroll thành công vào Othela | 2 |
| Agent clone/pull được repo từ Gitea | 2 |
| Agent chạy được dry-run và tạo file `.planed` | 2 |
| Kết quả về được tới Othela | 2 |

### B. Phát hiện đúng (10 điểm)

| | Điểm |
|---|---|
| Round 0: không có false positive | 3 |
| Round 1: bắt được cả 4 drift | 4 (1đ/cái) |
| Round 1: ký hiệu (`+ - ~ ↻`) đúng | 2 |
| Không báo drift ở task không liên quan | 1 |

### C. OX — trải nghiệm operator (10 điểm)

Chấm phần này bằng cảm giác thật của anh lúc nhìn màn hình, không phải bằng lý thuyết.

| | Điểm |
|---|---|
| Nhìn 5 giây là biết node ổn hay không | 3 |
| Output của node sạch đủ ngắn (không cuộn màn hình) | 2 |
| Drift hiện rõ *cái gì* đổi, không phải chỉ "changed" | 2 |
| `?` của task 7 hiện rõ và có giải thích | 2 |
| Không dump JSON thô của Ansible vào mặt operator | 1 |

### D. Hành vi khi hỏng (10 điểm)

| | Điểm |
|---|---|
| Round 3: dry-run gãy thì dừng, không chạy tiếp | 3 |
| Trạng thái lỗi phân biệt được với trạng thái drift | 3 |
| Thông báo lỗi nói được phải làm gì tiếp theo | 2 |
| Round 4: poll đúng interval, và nhìn được lần poll cuối | 2 |

**Tổng: 40**

---

### Cách hiểu điểm

Đọc kỹ. Đây là phần chống lại phản xạ tự đánh giá của anh.

| Điểm | Nghĩa thật |
|---|---|
| **0–5** | Chưa qua được bước enrollment. Không phải "dự án thất bại" — là "buổi tối nay dành cho enrollment". Đây là kết quả phổ biến nhất của lần chạy đầu tiên, ở mọi dự án. |
| **6–14** | **Đây là vùng bình thường cho một first light.** Nó chạy, nó có ý kiến, ý kiến sai nhiều. Anh vừa đổi một nỗi sợ mơ hồ lấy 8–12 cái bug có tên. Đó là một buổi tối cực kỳ hiệu quả. |
| **15–25** | Cái lõi hoạt động. Phần lớn điểm mất sẽ nằm ở mục C — tức là log xấu, không phải logic sai. Log xấu sửa nhanh hơn nhiều so với anh tưởng. |
| **26–34** | Xa hơn mức một dự án solo một năm tuổi thường đạt được ở lần chạy đầu. Nếu ra kết quả này, kiểm tra lại xem có chấm rộng tay không. |
| **35–40** | Kiểm tra lại. Hoặc anh chấm sai, hoặc anh vô tình chạy nó ở đâu đó trước rồi. |

**Ngưỡng để đêm nay được tính là thành công: agent chạy xong một lần và in ra thứ gì đó.** Điểm số chỉ là để có backlog. Không có điểm sàn nào để "đạt", vì không có ai chấm ngoài anh.

Một điều cuối, ghi ra đây để lát nữa anh đọc lại: **Helvilette chưa hề được cho cơ hội để chạy đúng.** Bất cứ cái gì gãy đêm nay đều gãy vì chưa ai từng chạy nó, chứ không phải vì nó không thể chạy được. Đó là hai chuyện hoàn toàn khác nhau, và lúc 11 giờ đêm anh sẽ có xu hướng nhầm chúng với nhau.

---

## 7. Sau khi xong

1. Dán toàn bộ output vào chat. Kể cả — nhất là — phần xấu.
2. Chưa sửa gì cả. Phân loại trước.
3. Snapshot lại các VM ở trạng thái sau test, tên `first-light`. Đây là baseline để so lần sau.
4. Đi ngủ. Việc sửa là của ngày mai.
