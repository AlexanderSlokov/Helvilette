# Helvilette: lộ trình theo Doctrine

Oct 11, 2026 · @Aleksandr Slokov

## Tầm nhìn

Helvilette là NGINX của cấu hình máy chủ: thứ NGINX lẽ ra đã trở thành nếu không ai bán lại cho người canh máy chủ những món đồ giúp họ sống sót. Bản miễn phí của NGINX trao chính nó, bản trả phí giữ lại kiểm tra sức khoẻ chủ động, đổi upstream không cần reload và bảng theo dõi trực tiếp. Helvilette cam kết không đi bắt bớ nguời vận hành như vậy.

Helvilette nên đạt được các tiêu chí sau

1. **C****ác ****t****í****nh ****n****ăng ****c****ốt ****lõi**** ****v****ề**** ****m****ặt ****v****ận h****ành ****không bao giờ nằm sau paywall****:** Trạng thái từng máy, halt triển khai kèm theo lý do, quay về commit cũ, immutable audit log luôn miễn phí.
2. Useful log: Có đủ file, dòng, task, commit, và theo các đề mục sau: cái gì hỏng, ở đâu, phần mềm đã làm gì, phần mềm chưa làm gì, một gợi ý cho 80% hành động nên làm tiếp theo.
3. **Không có bẫy kế thừa ngầm****:** Có lệnh in ra cấu hình hiệu lực cuối cùng (như `nginx -T`), và lệnh giải thích tính khả dụng của playbook áp lên máy này.
4. ** ****K****hông ****b****reak**** ****us****e****r s****pace****:**** `**helvilette.yml` và định dạng biên bản có số phiên bản, chỉ được thêm trường. Hành vi mặc định không đổi trong các bản vá.
5. **Không cần ****o****rc****hes****tra****tor** **để sống.** Agent độc lập giờ đây sẽ là tiêu chuẩn của dự án. Server phục vụ orchestrate quy mô lớn sẽ là phần cộng thêm.
6. **Giấy phép mở**** ****là bảo hiểm cho khẩu súng.** Đề xuất giữ Apache-2.0. Bài học từ NGINX là giấy phép mở cho phép khẩu súng sống tiếp qua các bản tách, kể cả khi chủ sở hữu đổi hướng.

## Tổng quan

&#91;embedded content: Lộ trình Helvilette · 4 giai đoạn, 4 cổng kiểm\]

Giai đoạn 1 là trọng tâm hiện tại.&#32;

Giai đoạn 3 chỉ mở khi có một đội thật cần phối hợp nhiều máy.

## Giai đoạn 0: Đầu tư cho Agent

Mục tiêu là tách agent khỏi Othela, tự thân Helvilette phải hoạt động được như Ansible-pull là tối thiểu.

- [x] Viết ADR "Agent độc lập, Othela tuỳ chọn": ADR-0008, ngày 14/10/2026.
- [ ] Đóng băng Othela: giữ mã trong repo, ngừng phát triển. README và BACKLOG tuyên bố đang đại tu (ADR-0008 D8).
- [ ] Gỡ khỏi đường chạy của agent: long-poll tới Othela, SQLite, đường hầm Chisel, và mọi kết nối mạng ngoài git và Kallisto.
- [ ] Kiểm kê bộ test hiện có (Ginkgo, Testcontainers) để giữ lại cho giai đoạn 1.

**Crit****e****r****ia****:** agent chạy trọn một vòng kéo git, chạy thử, áp dụng, ghi biên bản, vận hành độc lập.

### Trinh sát ansible-pull

Trước khi viết thêm cho agent, dùng thử `ansible-pull` đủ lâu để biết chính xác nó làm được gì và thiếu gì. Đây là đối thủ gần nhất, và cũng là mức sàn: agent ít nhất phải làm được mọi thứ `ansible-pull` làm.

- [ ] Dựng `ansible-pull` chạy bằng systemd timer trên 2–3 VM, với repo playbook thật, trong ít nhất một tuần.
- [ ] Thử các tính năng có sẵn: `--sleep` (chờ ngẫu nhiên), `--only-if-changed`, `--check`, `--checkout` theo nhánh hoặc commit, `--verify-commit`, và cách nó tự chọn `<hostname>.yml` / `local.yml`.
- [ ] Gây lỗi có chủ đích: mất mạng giữa chừng, rút điện, playbook lỗi, hai lần chạy chồng nhau. Ghi lại nó phản ứng ra sao và để lại dấu vết gì.
- [ ] Lập bảng so sánh: tính năng · `ansible-pull` có hay không · Helvilette xử lý thế nào.

Mỗi dòng trong bảng nhận một trong ba quyết định:

| Quyết định | Khi nào |
| --- | --- |
| Dùng lại | `ansible-pull` đã làm tốt: agent gọi thẳng nó, hoặc học y cách làm (ví dụ `--sleep`). |
| Bọc ngoài | Làm được nhưng thiếu biên bản, thiếu trạng thái: agent bọc bên ngoài để bổ sung. |
| Tự viết | `ansible-pull` không có: chạy đúng một lần, biên bản trên đĩa, `plan`, halt. |

Câu hỏi mở: agent nên gọi thẳng `ansible-pull` làm engine (như hôm nay gọi `ansible-playbook`), hay chỉ học thiết kế của nó? Trả lời sau khi trinh sát xong, dựa trên bảng.

**Criteria:** bảng so sánh hoàn tất, mỗi dòng có quyết định.

## Giai đoạn 1: Agent 1.0

Agent 1.0 là một `ansible-pull` được nâng cấp chỉn chu: tự kéo, tự kiểm tra với playbook được cung cấp, tự ghi biên bản, và chạy được một mình.

### Cơ cấu

- [ ] Tiến trình thường trực dưới systemd (`Restart=always`), gọi `git fetch` định kỳ, chờ ngẫu nhiên để cả đàn máy không kéo cùng lúc. Thường trực vì agent phải chủ động đi kiểm tra probe. Mọi trạng thái quan trọng nằm trên đĩa, ghi atomic, nên bị `kill -9` lúc nào cũng không hỏng (ADR-0008 D4).
- [ ] URL repo và nhánh nằm trong cấu hình cục bộ của agent (`agent.yaml`, đặt bằng cloud-init), giống `ansible-pull -U -C`; không cần server "trao" repo cho agent (ADR-0008 D2).
- [ ] Chỉ chấp nhận commit ký bằng khoá SSH có trong danh sách trên máy. Bắt buộc tối thiểu hai khoá: một khoá dùng hằng ngày, một khoá cất offline (ADR-0008 D9).
- [ ] Đọc `helvilette.yml`: playbook nào áp cho máy này theo nhãn, playbook nào được tự áp dụng, playbook nào chỉ lập kế hoạch.
- [ ] Luôn chạy thử trước. Chạy thử hỏng thì dừng, máy giữ nguyên trạng thái cũ.
- [ ] Cảnh báo khi tỉ lệ bước không chạy thử được vượt ngưỡng; quyền "cứ chạy đi" là một dòng trong yaml, nên cũng được ký.
- [ ] Khoá chống hai lần chạy chồng lên nhau.
- [ ] Lấy bí mật từ Kallisto qua lookup Vault có sẵn của Ansible.

### Safe-guards

- [ ] Dòng đánh dấu trên mỗi file agent giữ, theo quy ước `ansible_managed`: do Helvilette quản lý, playbook nào, muốn sửa tay thì halt trước.
- [ ] Lời chào khi đăng nhập: máy đang được giữ trạng thái, bao nhiêu file, bao nhiêu service, lần chạy gần nhất
- [ ] Task `shell` và `command` được ghi là "các bước không thể biết chính xác tác động".
- [ ] `helvilette halt --reason` với danh mục lý do: bảo trì có kế hoạch, chữa cháy sự cố, sự cố hạ tầng hoặc mất điện, thử nghiệm, khác (bắt buộc ghi chú - giống windows server). `helvilette resume` để bỏ halt.
- [ ] Halt không tự hết hạn. Mỗi vòng, agent ghi một dòng cảnh báo vào syslog và lời chào đăng nhập hiện rõ đang halt bao lâu, vì lý do gì, do ai.

### Probe và playbook khắc phục

Tính năng lõi, thứ `ansible-pull` không có (ADR-0008 D7). `ansible-pull` biết cấu hình có khớp playbook không, nhưng không biết phần mềm còn sống không. Nhiều app không có công cụ đo downtime từ ngoài vào, cũng không chạy trong container để container runtime kiểm tra hộ.

- [ ] Liveness probe cho service systemd: HTTP, TCP, exec. Bật theo từng service trong `helvilette.yml`, kèm lý do.
- [ ] Probe hỏng thì chạy thử playbook của service đó trước, để biết cái gì đã thay đổi.
- [ ] Playbook khắc phục chỉ chạy khi service đó đã opt-in. Hỏng thì lùi dần (backoff), không chạy vòng lặp.
- [ ] Mọi lần khắc phục được ghi vào biên bản và journal.
- [ ] Readiness, dùng làm điều kiện cho rolling update, là việc của Othela ở giai đoạn 3.

### Diff on instance

Các lệnh quản trị + halt và resume: `status`, `plan`, `config` (cấu hình hiệu lực), `explain <playbook>`, `footprint`. Không có lệnh `apply`: công cụ cứu hộ là `ansible-playbook`.

### Các ràng buộc về trải nghiệm vận hành

- [ ] Định dạng biên bản v1, có trường số phiên bản, khi nâng cấp cần có lộ trình cho cả file config này.
- [ ] Log jsonl từng task trên đĩa; Alloy hoặc công cụ tương tự chuyển đi, agent không tự chuyển.
- [ ] Bản tóm tắt cho người đọc theo dạng `+ - ~`; khi lỗi, tự mở rộng task hỏng và một số task trước nó.
- [ ] Thông báo lỗi theo chuẩn bốn ý.

### Đóng gói

- [ ] Một file chạy Go duy nhất.
- [ ] Ansible là đạn: ghim phiên bản và đóng gói sẵn, người dùng không tự đi mua.
- [ ] Cài bằng một dòng trong cloud-init hoặc nướng sẵn bằng Packer.

### Bộ test kiểu SQLite

| Điều kiện cực đoan | Hành vi bắt buộc |
| --- | --- |
| Agent bị `kill -9` giữa lúc đang áp dụng | Lần chạy sau phát hiện lần trước dở dang, ghi vào biên bản, chạy lại từ đầu |
| Mất điện giữa lúc đang áp dụng | Như trên; không có file trạng thái nào bị hỏng |
| Đầy đĩa | Dừng, báo lý do, không ghi đè biên bản cũ |
| Git server không tới được | Giữ trạng thái đã áp dụng, vẫn sửa drift theo commit cuối cùng đã được xác minh |
| Kallisto không tới được | Không áp dụng task cần bí mật, báo rõ task nào bị bỏ qua |
| Commit sai chữ ký | Từ chối, giữ commit cũ, báo commit nào bị từ chối |
| Bản checkout bị hỏng | Phát hiện, clone lại, ghi vào biên bản |
| Đồng hồ máy bị lệch | Không làm hỏng thứ tự biên bản; ghi lại độ lệch |
| Commit mới đến giữa lúc đang chạy | Chạy xong commit hiện tại, commit mới vào vòng sau |

### Cổng ra

| Bài thử | Điều kiện đạt |
| --- | --- |
| Homelab | Tác giả chạy agent thật |
| Bài thi trong bùn | Đàn máy thử bị phá ngẫu nhiên (`sshd_config`, xoá user, hạ phiên bản gói, tắt service); agent phát hiện và kéo về đúng khai báo, không lỗi |
| Concorde | 74 phút tải cao, có một commit vá nóng giữa chừng; không lỗi, không chết, giữ cho server tiếp tục hoạt động |

## Giai đoạn 2: public beta

Mục tiêu là để một người vận hành không quen tác giả tự cài, tự dùng và tự sửa được.

- [ ] Tài liệu kiểu thẻ tra cứu: mỗi khoá trong `helvilette.yml` có cú pháp, giá trị mặc định và chỗ được dùng, như tài liệu chỉ thị của NGINX.
- [ ] Quickstart chuẩn, dài không quá một trang: "Bạn có repo Ansible? Cool."
- [ ] Ma trận hỗ trợ được thử thật: Ubuntu, Debian, RHEL; VPS trong nước, Proxmox, EC2, GCP.
- [ ] Hướng dẫn tạm dừng và cứu hộ bằng `ansible-playbook`, viết cho người đang chữa cháy lúc 2 giờ sáng.

**Cổng ra:** ít nhất một người vận hành ở một công ty khác, không quen tác giả, tự tìm đến, tự cài, và báo lỗi bằng issue từ môi trường của họ.

## Giai đoạn 3: Othela tuỳ chọn

Othela chỉ tiếp tục phát triển có **ít nhất một đội thật có nhu cầu thật về phối hợp nhiều máy.**

**Luật bất biến:** Othela chỉ đọc biên bản do agent ghi ra, và chỉ ghi vào git. Bên trong agent không có "chế độ Othela". Rút Othela ra thì mọi agent vẫn tuần tra như cũ.

Năng lực dự kiến:

- **Bản đồ đội máy:** máy nào lệch khỏi khai báo, máy nào đang halt, vì lý do gì, từ bao giờ.
- **C**ông việc nhiều máy, có thứ tự, chạy bằng `ansible-playbook` kiểu đẩy với đầy đủ `serial`, `delegate_to`, `run_once`. Bộ não điều phối là Ansible; Othela chỉ mở đường hầm ngược cho máy sau NAT, và đóng cửa khi nhiệm vụ kết thúc.

**Bài tốt nghiệp:** dựng k3s-ansible trên một đàn máy, và một bài Concorde riêng cho Othela.

## Định hình paywall

M**ột người vận hành đơn độc có cần thứ này để sống sót qua sự cố không?** Có thì miễn phí. Chỉ tổ chức có quy mô mới cần thì được bán.

| Miễn phí, mãi mãi | Được bán |
| --- | --- |
| Toàn bộ agent và mọi lệnh trên máy | Phân quyền cho nhiều đội (RBAC) |
| Halt kèm lý do, quay về commit cũ | SSO, LDAP, xác thực tuỳ biến |
| Thông báo lỗi, biên bản, log; nhật ký kiểm toán bất biến | Chứng nhận nhật ký đạt chuẩn kiểm toán (việc của tập đoàn, dời lại) |
| Dấu vết cơ bản: ai đổi gì, lúc nào, lý do halt | Hồ sơ tuân thủ: FIPS 140, SOC 2, ISO 27001 |
| Bản đồ đội máy cơ bản trong Othela | Điều phối nhiều site, nhiều cụm |

Bản đồ đội máy cơ bản nằm ở cột miễn phí vì một người trông 50 máy cần biết máy nào đang lệch để sống sót. Đó chính là thứ F5 đã đặt sau paywall của NGINX.

### Không làm

- Không dựng hay dựng lại cụm. Đó là việc của Terraform và cloud-init.
- Không thay thế Ansible, không có lệnh `apply` riêng.
- Không cần giao diện đẹp. Được phép xấu, không được phép làm khó người vận hành.
- Không tự bỏ halt, không bao giờ tự đảo ngược quyết định của người vận hành.

### Rủi ro

- **Ansible và Python trên mọi máy** là bộ phận nặng nhất. Nếu không ghim phiên bản, mỗi máy sẽ chạy một kiểu.
- **Điểm mù của chế độ chạy thử:** task `shell` và `command` thường bị bỏ qua, nên Helvilette phải báo cáo rõ ràng "check 10 task, 8 task xác nhận chạy đúng, 2 task không biết - lý do là 2 lệnh shell".
- **Khoá ký commit của người vận hành:** nằm ở đâu, và mất thì làm gì. Đây là chốt bảo vệ duy nhất, vì ai đẩy được vào repo là người đó cấp được quyền tự chạy.
- **Cảnh báo trong syslog không ai đọc** nếu không được gom về.
- **Trên cloud phải đối đầu với đồ của chủ đất** (AWS Systems Manager, công cụ quản lý cấu hình của GCP). Lợi thế duy nhất là cùng một cách làm ở mọi nơi.
