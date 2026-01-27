# Helvilette Future Roadmap & TODOs

## 1. Quản lý Ansible Playbook (The Core Engine)
Hiện tại đang hardcode string trong Go. Cần chuyển sang cơ chế quản lý file thực thụ.

### Yêu cầu:
- **GitOps-driven:** Othela phải biết tự pull Playbooks từ một Git Repository (GitHub/GitLab) về `othela_data/playbooks`.
- **Ansible Galaxy Support:** Tự động phát hiện file `requirements.yml` trong repo để cài đặt các Roles/Collections cần thiết (`ansible-galaxy install -r ...`).

### Đề xuất giải pháp (Technical Proposal):
1.  **Repo Watcher (Go Routine):** Một thread chạy ngầm trên Othela, định kỳ `git pull` từ remote repo.
2.  **Versioning:** Mỗi lần commit mới sẽ có hash SHA. Othela dùng SHA này làm "Job Version" để đảm bảo các Agent update lên phiên bản mới nhất.
3.  **Local Cache:** Othela cache playbook ra đĩa. Khi Agent hỏi, Othela đọc file từ đĩa gửi đi (hoặc nén trả về URL download nếu file lớn).

## 2. Agent Intelligence (State Awareness)
- **Drift Detection:** Thay vì chạy đè (`force`), Agent nên chạy `check_mode` (-C) trước. Nếu kết quả là `changed=0`, báo "Green". Nếu `changed>0`, mới chạy thật (hoặc báo vàng chờ duyệt).
- **Security:** Triển khai mTLS cho kết nối gRPC/HTTPs giữa Agent và Othela.

## 3. UI/Dashboard
- Cần một Web UI đơn giản hiển thị:
    - Danh sách Node.
    - Trạng thái Job gần nhất.
    - Log realtime (Stream qua WebSocket).
