# Software Design: Follow the Soviet's way

Oct 11, 2026 · @Aleksandr Slokov

**Tác giả:** Đinh Tấn Dũng (ORCID: 0009-0003-1374-7525), nhà nghiên cứu độc lập, TP. Hồ Chí Minh.

## Tóm tắt

Bài viết này đề xuất một học thuyết thiết kế phần mềm lấy cảm hứng từ học thuyết chiến tranh đã định hình nên thiết kế của các vũ khí Liên Xô trong giai đoạn 1940-1980. Đối tượng phục vụ là người vận hành hệ thống ở các tổ chức nhỏ, những người phải chịu trách nhiệm với sự ổn định của hệ thống nhưng không có quyền lực chính trị công sở cần thiết, và làm việc dưới áp lực với rất ít công cụ.

Học thuyết này nhấn mạnh một luận điểm: **phần mềm ****p****h****ải ****đ****ạt ****được**** ****s****ự nh****ấ****t ****quán**** v****à ****đ****ộ ****tin c****ậy**** ****c****ao**** ****n****h****ất ****d****ưới ****m****ọi ****đ****i****ều ****ki****ện ****của m****ôi**** tr****ường**** ****v****à mọi**** thao t****ác ****đ****úng**** ****đ****ắ****n****,**** ****c****ũng ****như ****đ****ề k****háng ****đ****ược tr****ước ****t****hao t****á****c** **s****ai s****ó****t** **c****ủa con ng****ười**** ****(****m****ột ****kh****ẩu**** s****úng**** ****đã**** ****bắn được lúc bình thường thì phải bắn được lúc có địch****)****.** Từ luật này rút ra các nguyên lý về hành vi khai báo rõ ràng, độ phức tạp quản lý được, giao diện bền vững và tư thế làm việc của người vận hành. Bài cũng nêu hai tiêu chí đánh trượt và ba bài thử.

Học thuyết được kiểm chứng trên ba công cụ của hệ sinh thái Naughtian: Kallisto, Helvilette và Naught.&#32;

Một phát hiện quan trọng là việc ép một khuôn mẫu duy nhất lên mọi công cụ là sai (không thể lấy sự thành công của AK47 gò ép làm khuôn mẫu cho mọi phần mềm). Thứ cần mượn là tư duy thiết kế vũ khí của Liên Xô là phương pháp để định hình nên tiêu chí cho phần mềm, phù hợp với mục đích mà nó phục vụ.

## Đối tượng phục vụ

Kỹ sư Liên Xô luôn thiết kế vũ khí cho một mẫu người dùng cụ thể: nếu là PPsh41 hay AK47, người dùng của chúng sẽ là cậu lính nghĩa vụ mới huấn luyện vài tuần, đứng trong bùn và giá rét. Học thuyết này cũng bắt đầu bằng việc gọi  tên đối tượng phục vụ của mình.

### 1.1. Người vận hành hệ thống

Đôi khi đó là một nhóm rất nhỏ, có khi chỉ một người kiêm nhiều vị trí: devops, hạ tầng, quản trị hệ thống và cơ sở dữ liệu. Họ chịu áp lực lớn về thời gian khôi phục dịch vụ, từ một cấp quản lý chỉ muốn sự cố được giải quyết "càng nhanh càng tốt".

Khoảnh khắc tệ nhất của họ trông như sau:

**Hai mặt trận cùng lúc****:** Một bên là hệ thống đang hỏng. Bên kia là cấp trên không chuyên kỹ thuật nhưng cố gắng can thiệp vào quy trình xử lý sự cố.

**Hệ thống lai****:** Một "distributed monolith" nửa container, nửa kế thừa, log rải rác khắp nơi.

**N****guyên**** ****nhân**** s****ự cố****:**** **Theo Google SRE, khoảng 70% sự cố mất dịch vụ bắt nguồn từ thay đổi trên hệ thống đang chạy, không phải từ code chết \[Beyer và cộng sự, 2016\]. Thay đổi cấu hình là một dạng như vậy: dịch vụ vẫn chạy nhưng chạy sai. Dịch vụ có nhiều bản sao thì phải SSH vào từng máy để sửa cấu hình.

**Đồ nghề tối thiểu****:** Trong tay có một giao diện quản lý container và gần như không có gì khác. Không có rolling update, vì phần mềm được triển khai vốn không được thiết kế để làm được rolling update, chỉ có thể thực hiện blue-green  deployment có downtime.

**Sau sự cố là đổ lỗi****:** Môi trường không trao cho người vận hành vai trò chỉ huy sự cố. Thứ chờ họ là bản phân tích nguyên nhân, quy trách nhiệm và kỷ luật.

### 1.2. Nỗi đau chung

Khái quát lên, người vận hành tại các SME thường là những người phải **ch****ị****u** **trách nhiệm nhưng không có**** th****ực q****u****y****ền**** ****v****à ****đ****ư****ợc ****đ****ối ****x****ử**** ****t****ương ****x****ứng v****ới ****tr****ách ****nhi****ệm** **đó**. Hệ thống do người khác quyết định, phần lớn sự cố đến từ thay đổi trên hệ thống đang chạy như thay đổi cấu hình, và khi hỏng thì người khác chen vào quyết định. Sự chú ý của người vận hành vì thế đã bị chia đôi: một nửa cho hệ thống, một nửa cho con người.

Từ đó, tác giả muốn chỉ ra một định nghĩa: **công cụ nào lấy thêm dù chỉ một phần sự chú ý ****của**** ****m****ột ng****ười ****vận ****h****ành**** ****k****h****ỏi vấ****n** **đề**** đang**** x****ảy r****a**** l****à m****ột c****ông c****ụ t****ồi****.**

### 1.3. Các yêu cầu của một phần mềm hạ tầng đáng tin cậy

Trước khi vẽ thiết kế cho một vũ khí, kỹ sư Liên Xô nhận một bản yêu cầu chiến thuật – kỹ thuật. Tuân thủ quy trình trên, một bản yêu cầu cho phần mềm phục vụ operator sẽ có tối thiểu các yêu cầu kỹ thuật sau:

1. **W****i****t****hst****and**** ****u****n****in****t****e****n****d****ed**** ****re****s****tart**** ****/ ****re****boo****t****:** Không có concruptable state để bị hư hỏng khi bị tắt ngang. Hoặc bản thân phần mềm phải được thiết kế để chống chịu trước các đợt restart / reboot bất ngờ, mất điện.
2. **E****xpla****i****n****able****:** Công cụ phải giúp người vận hành chứng minh được trong khi sự cố xảy ra, anh ta đã có các thao tác đúng đắn để giải quyết sự cố (best efforts taken). Và log của phần mềm phải hỗ trợ operator chứng minh được nỗ lực và trình độ của anh ta nhanh nhất, tường minh nhất với người khác.
3. **A****u****d****it****a****b****le****:**** **Ai đổi gì, lúc nào, ai duyệt. Sau sự cố, bằng chứng là thứ che chắn cho người vận hành. Phần mềm phải phục vụ được quá trình truy vết quyết định để đảm bảo blameless port-monterm có thể diễn ra hợp lệ (actionable post-monterm by design).
4. **C****e****n****tra****li****zed ****con****fig****u****ra****tion****:**** ****p****h****ục ****v****ụ** **q****u****ản tr****ị v****à ****kiểm**** to****án ****t****ập ****t****rung****.**
5. **D****is****pos****able ****an****d**** ****r****e****pla****c****e****a****b****le****:** Gắn được vào bên cạnh công cụ hiện tại, không đòi dọn nhà. Required minimal effort to take care off, no hidden side-effects.

## 2. Related works

### 2.1. Lược bàn về thiết kế vũ khí Xô Viết

Cùng một phương pháp luận về sự đơn giản, nhưng học thuyết chiến tranh của Liên Xô cho ra những cỗ máy có độ phức tạp chênh nhau rất xa:

| Cỗ máy | Nhiệm vụ | Bài học cho phần mềm |
| --- | --- | --- |
| AK-47 / AKM | Súng cấp phát cho bộ binh, dùng hằng ngày trong mọi điều kiện | Ít chi tiết, bảo trì không cần dụng cụ, chịu được môi trường tồi tệ, sản xuất được ở mọi nơi (reproducible at everywhere) |
| SVD (Dragunov) | Súng thiện xạ cấp tiểu đội | Một thiết kế xuất sắc giải quyết cho một việc riêng. |
| MiG-21 | Máy bay đánh chặn: lên nhanh, đánh nhanh, về nhanh. Máy bay đơn giản được vì hệ thống radar và dẫn đường đặt dưới mặt đất | Bản thân phần mềm cần phải đơn giản, những việc phức tạp mới cần huy động các thành phần khác để đạt được khả năng phức tạp. |
| S-75 Dvina | Hệ thống tên lửa phòng không chống máy bay ném bom tầm cao | Được phép phức tạp vì mối đe doạ là có thật, nhưng mỗi bộ phận vẫn chắc chắn và vận hành được |

### 2.2. Bài học từ MiG-21

MiG-21 gần như trơ trụi: radar yếu, tầm bay ngắn, điện tử tối thiểu. Nó đơn giản được vì hệ thống radar và sở chỉ huy dưới mặt đất làm phần việc khó: tìm mục tiêu, tính góc đánh, gọi phi công lên đúng lúc. Lối đánh vào nhanh, ra nhanh của phi công Việt Nam dựa chính vào hệ thống mặt đất đó.

Bài học là kỹ thuật Liên Xô không loại bỏ độ phức tạp. Nó **đặt độ phức tạp vào chỗ bảo trì được**. Đồng thời, máy bay vẫn bay được và phi công vẫn đánh được bằng mắt khi mất liên lạc với mặt đất.

### 2.3. Sai lầm trong quá trình phát triển lý thuyết

Ý tưởng ban đầu của white paper này ép mọi phần mềm hạ tầng phải mang các đặc tính của một khẩu AK. Đó là một sai lầm cần được thừa nhận như một phần của white paper. Ép mọi thứ thành AK vì ngưỡng mộ AK là duy ý chí: lấy hình dạng của cỗ máy mình thích thay cho hình dạng của nhiệm vụ. Nhưng biện chứng cần được thực hiện hai chiều. Xây cả một hệ thống S-75 khi chưa thấy máy bay địch cũng là duy ý chí. Liên Xô xây S-75 vì máy bay do thám và máy bay ném bom là có thật. Câu hỏi đúng phải là: **mối**** v****ấn ****đ****ề ****c****ốt l****õi**** ****nằm ở đâu, và là của ai?**

## 3. Hình mẫu sai lầm

### 3.1. Bảng điều khiển chiếm quyền máy chủ

Kiểu mẫu được nêu tên trong phần phân tích này là loại bảng điều khiển hosting cài lên máy chủ Linux rồi chiếm quyền điều khiển nó. DirectAdmin là một ví dụ, người từng dùng các sản phẩm cùng loại sẽ nhận ra những vấn đề sau:

| Biểu hiện | Nguyên lý đối diện |
| --- | --- |
| Script ẩn, tác dụng phụ chồng chéo; ý đồ của phần mềm không được thể hiện ra, tự thực hiện các hành động ngầm định, làm sai rồi quy lỗi cho người vận hành | Không làm gì sau lưng; mọi ý đồ phải được khai báo |
| Chiếm cả máy, bất hợp tác với quản trị viên Linux; ôm đồm những việc systemd đã làm tốt | Đứng cạnh, không thay thế; dùng lại hệ điều hành đã có |
| Log phân mảnh. Nuốt lỗi, ba lỗi khác nhau trả về cùng một thông báo khó hiểu | Mỗi lỗi một lời, nói đúng chỗ, bằng tiếng người |
| API cũ mới lẫn lộn, không có Terraform provider, GitOps làm cho có, không có hành động sau triển khai, không có cách cài chuẩn | Giao diện là băng đạn: phải là phần chuẩn và bền nhất |
| Nuôi ảo tưởng "đã làm được mọi thứ" ở cấp quản lý, biến người vận hành thành người gánh lỗi | Phần mềm phải làm năng lực của người vận hành hiện ra được |

Vấn đề thứ năm hiếm khi được viết ra: **thiết kế phần mềm có chiều chính trị**, vì nó quyết định ai trong phòng trông có vẻ có năng lực. Kiểu phần mềm này được thiết kế cho người ký hợp đồng mua, không phải cho người cầm nó lúc 2 giờ sáng. Mọi cái sai về cách làm đều chảy ra từ đó.

### 3.2. Hình mẫu đúng đắn

Những phần mềm tác giả xem là làm đúng tinh thần này: Portainer, Dozzle, Kamal của 37signals, Prometheus, Grafana, GitLab Omnibus CE. Chúng có chung ba đặc điểm:

1. **C****om****p****re****hen****si****v****e**** ****d****o****c****u****m****enta****t****ion****:**** **Tài liệu là một bộ phận không thể tách rời của phần mềm. Các phần mềm hạ tầng xuất sắc có bộ tài liệu vận hành cực kỳ tường minh và đầy đủ. Từ đó, người vận hành mới có thể cấu hình và sử dụng phần mềm đúng theo ý đồ thiết kế, cũng như có đủ tư liệu để xử lý các vấn đề phát sinh.
2. **T****he ****gol****den ****p****at****h****:**** **Người mới làm theo một cách hiển nhiên là chạy được, không phải mò mẫm.
3. **S****afe ****d****e****fault****s****:**** **Phần nào chưa được cấu hình thì không có gì bất ngờ xảy ra ở đó.

### 3.3. Vì sao GitLab Omnibus vẫn được xem là một phần mềm hạ tầng tốt dù vô cùng phức tạp

GitLab Omnibus có rất nhiều vấn đề mà hình mẫu sai trong paper này chỉ ra: tự chạy nginx, PostgreSQL, Redis và hàng loạt dịch vụ riêng. Khác biệt nằm ở chỗ nó **chiếm có khai báo**. Mọi thứ nằm gọn trong một thư mục, cấu hình nằm trong một file, và chỉ thay đổi khi người vận hành tự chạy lệnh cấu hình lại.

Kết luận: một phần mềm bị paper này kết luận mà một phần mềm tệ khi nó có quá nhiều implicit intends and actions. Độ phức tạp của một phần mềm vẫn là một đặc điểm không thể trách khỏi, miễn là nó quản lý được.

## 4. Học thuyết

### 4.1. Luật số 0

> **Bắn được lúc bình thường thì phải bắn được lúc có địch.**

Luật này có ba hệ quả:

1. **Chỉ có hai trạng thái:** chạy đúng như đã tuyên bố, hoặc từ chối và nói rõ vì sao. Không có trạng thái thứ ba "chạy nhưng sai", vì đó chính là kẻ địch số một của người vận hành.
2. **Không làm gì sau lưng.** Mọi hành động đều có dấu vết. Không có tác dụng phụ ẩn nào chờ lộ ra lúc chữa cháy.
3. **Gần như không cần chăm.** Thứ cần chăm thì sẽ bị bỏ bê, và nó hỏng đúng lúc bị bỏ bê.

### 4.2. Các nguyên lý

Mỗi nguyên lý gắn với một hình ảnh cơ khí để dễ nhớ.

1. **Nhiệm vụ trước, hình dạng sau.** Lấy hình dạng từ công việc, không lấy từ cỗ máy mình ngưỡng mộ.
2. **Đặt bộ não ở chỗ sự thật đã nằm sẵn.** Nếu trạng thái mong muốn đã sống trong git, đừng dựng thêm một server giữ bản sao thứ hai của sự thật.
3. **Mượn cơ cấu đã được chứng minh.** Khoá nòng xoay của AK không phải phát minh mới. Chỉ phát minh cách ghép, không phát minh phần lõi.
4. **Băng đạn là thứ bền nhất.** Định dạng dữ liệu và giao diện phải chuẩn, có số phiên bản, chỉ được thêm trường. Code xung quanh là đồ thay được. Nói chung tiếng với hệ sinh thái có sẵn để lắp lẫn được.
5. **Chừa khe hở giữa các bộ phận, nhưng nòng súng phải mạ chrome.** Các thành phần nói chuyện qua định dạng đơn giản, không phải khớp nhau từng li. Nhưng chỗ chạm vào thế giới bên ngoài thì phải kiểm tra nghiêm ngặt.
6. **Dồn sức dư vào chỗ hay hỏng.** Không rải đều sự cẩn thận. Tìm điểm hỏng thường xuyên nhất rồi dồn dư thừa vào đó.
7. **Tháo dã chiến không cần dụng cụ.** Một người vận hành mới phải tháo ra, hiểu và lắp lại được trong một ngày.
8. **Ống lót tay.** Thiết kế cho đúng khoảnh khắc người dùng quên mất công cụ đang tồn tại. Không đổ lỗi cho người cầm vào nòng súng nóng.
9. **Không bao giờ tự đảo ngược quyết định của con người.** Công cụ chỉ nhắc, và nhắc mãi.
10. **Phức tạp chỉ được phép ở thứ không chạy khi không có việc gì xảy ra.**
11. **Công cụ cứu hộ là thứ người vận hành đã có.** Khi công cụ vắng mặt hoặc bị gỡ ra, người vận hành vẫn sửa được hệ thống bằng đồ nghề của mình.
12. **Được phép xấu, không được phép làm khổ.** Hy sinh vẻ bóng bẩy và trải nghiệm "wow". Không bao giờ hy sinh tư thế làm việc của người vận hành và người bảo trì. Lọc dầu phải nằm ở chỗ tay với tới.

### 4.3. Độ phức tạp quản lý được

Độ phức tạp được phép khi thoả cả bốn điều kiện:

1. **Đoán trước được.** Cùng một cấu hình luôn cho cùng một kết quả.
2. **Có vách ngăn kín nước.** Hỏng một khoang chỉ ngập khoang đó, không lan dây chuyền tới mức tê liệt cả hệ thống.
3. **Tập trung một chỗ.** Cấu hình, dữ liệu và log, mỗi thứ nằm ở một chỗ.
4. **Nói tiếng người khi hỏng.** Stacktrace vào file log. Thứ hiện ra trước mặt người vận hành phải đủ bốn ý: cái gì hỏng, hỏng ở đâu, công cụ đã làm gì và **không** làm gì, nên làm gì tiếp.

Ví dụ một thông báo đúng chuẩn:

> Dừng ở task "cấu hình nginx" (playbook web.yml). Chạy thử báo `/etc/nginx/nginx.conf` không qua được `nginx -t`. Tôi chưa áp dụng gì, máy vẫn ở trạng thái của lần chạy trước. Kiểm tra commit `a1b2c3` trong repo.

### 4.4. Đạn và đồ phụ

Mọi phụ thuộc được chia làm hai loại:

- **Đạn** là thứ chính nhiệm vụ đòi hỏi. Một công cụ chạy playbook Ansible thì Ansible là đạn. Đạn vẫn phải được ghim phiên bản và đóng gói sẵn.
- **Đồ phụ** là thứ công cụ đòi chỉ để phục vụ bản thân nó, ví dụ một hàng đợi riêng hay một cơ sở dữ liệu riêng. Đồ phụ luôn bị nghi ngờ trước tiên.

## 5. Ba ca thực chiến

### 5.1. Kallisto: bản thiết kế đầu tiên đạt chuẩn

Kallisto giao bí mật tới từng máy mà không cần một cụm Vault luôn sẵn sàng. Sau đợt cắt giảm, nó chỉ còn là một tiến trình kéo file đã mã hoá về, rồi phục vụ chúng qua API tương thích KV v2 của Vault.

- **Ít chi tiết:** không cụm máy, không bỏ phiếu đồng thuận, không trạng thái phải chăm.
- **Băng đạn lắp lẫn:** ứng dụng nào đang đọc `VAULT_ADDR` cắm vào là chạy. Thay Kallisto bằng Vault thật thì ứng dụng không hề biết.
- **Thép dập:** các cấu trúc dữ liệu tự viết bị bỏ, thay bằng Rust an toàn tiêu chuẩn (`arc-swap`, `tokio`). Kallisto vẫn đạt khoảng 80.000 request mỗi giây khi bão hoà 2 nhân của một laptop phổ thông, dư sức nhiều lần so với nhu cầu của một tổ chức nhỏ.

Hệ quả đáng ghi: khi bỏ hết bộ phận tự chế, chỗ dễ vỡ không biến mất mà **dời sang danh sách phụ thuộc**. Đổi như vậy là hời, vì những thư viện đó có rất nhiều người canh. Nhưng đó cũng chính là loại mục mà Naught được sinh ra để nhìn.

Kallisto mới là **bản thiết kế** đạt chuẩn. Để thành vũ khí đạt chuẩn, nó cần được người khác dùng trong môi trường của họ.

### 5.2. Helvilette: từ HK416 về MiG-21

Helvilette đưa playbook Ansible về chạy ngay trên từng máy theo kiểu kéo, để người vận hành không phải giữ một rổ khoá SSH. Phiên bản đầu sao chép hình dạng của Kubernetes: một control plane (Othela), agent trên mỗi máy, SQLite làm kho trạng thái. Kết quả là một khẩu HK416: chính xác nhưng khó làm và cồng kềnh, đến mức tác giả không dám chạy nó ở homelab của mình.

Phân tích theo học thuyết:

1. **Phân loại nhiệm vụ.** Công việc chia làm hai loại. *Tuần tra*: mỗi máy tự giữ trạng thái của mình, hằng ngày. *Đột kích*: nhiều máy phối hợp có thứ tự, hiếm khi xảy ra. Trong điều kiện thực tế của tác giả, đột kích gần như không tồn tại: cụm k3s hỏng thì đốt đi và dựng lại bằng Terraform, và việc dựng cụm đã có công cụ riêng.
2. **Đặt bộ não ở chỗ sự thật nằm sẵn.** Bản khai báo đã nằm trong git, trong `helvilette.yml`. Agent không cần ai cầm tay chỉ việc, chỉ cần đọc được nhiệm vụ của mình.
3. **Agent là MiG-21.** Ở giai đoạn 1, agent là một phần mềm hoàn chỉnh, một `ansible-pull` được nâng cấp chỉn chu: kéo git định kỳ, kiểm chữ ký commit, đọc yaml, luôn chạy thử trước, chỉ áp dụng khi yaml cho phép, ghi biên bản. Agent là một tiến trình thường trực, vì nó phải chủ động đi kiểm tra sức khoẻ dịch vụ (probe) chứ không chỉ chờ lịch chạy: MiG đã cất cánh thì phải bay suốt nhiệm vụ. Bù lại, nó không giữ trạng thái quan trọng nào trong bộ nhớ, nên bị tắt ngang ở bất cứ lúc nào cũng không hỏng gì. "Không có tiến trình thường trực" là lựa chọn tuỳ phần mềm, không phải luật chung.
4. **Othela là mạng dẫn đường mặt đất.** Othela được hoãn sang giai đoạn 2, chỉ được thêm vào, không bao giờ trở thành thứ bắt buộc. Nó chỉ đọc những gì agent đã ghi ra và chỉ ghi vào git; bên trong agent không có "chế độ Othela". Bản Helvilette điều phối nhiều máy là một hệ hình khác, dành cho những đội có mối đe doạ thật, và cần một đơn vị thật để bắn thử.
5. **Ống lót tay.** Agent khai báo vùng chạm của mình: dòng đánh dấu trên mỗi file nó giữ, và lời chào khi đăng nhập cho biết máy đang được giữ trạng thái. Lệnh `helvilette halt --reason` có danh mục lý do sẵn và ghi vào biên bản. Halt không tự hết hạn; agent chỉ ghi cảnh báo vào syslog sau mỗi vòng.
6. **Công cụ cứu hộ là Ansible.** Playbook là playbook thường. Người vận hành luôn chạy trực tiếp được mà không cần tới agent.

Chỗ có thể gãy: danh sách vùng chạm chỉ lấy được từ kết quả các lần chạy thật. Các task `shell` và `command` có vùng chạm không xác định, và phải được ghi rõ là như vậy. Cảnh báo trong syslog chỉ có ích khi được chuyển tới nơi có người đọc.

### 5.3. Naught: kẻ quan sát

Naught nhìn sự mục rữa của hạ tầng nhưng không bao giờ hành động: không vá, không nâng cấp, không ghi vào hệ thống nó quan sát. Nó khớp học thuyết ở nhiều điểm: dữ liệu gốc là file bất biến, mọi thứ dẫn xuất đều xoá và dựng lại được, mọi kết luận đều đi kèm độ phủ.

Học thuyết đặt ra hai câu hỏi cho Naught:

- Ràng buộc "phức tạp bên trong, đơn giản bên ngoài" nên được chỉnh thành: bên trong đủ đơn giản để tháo được; độ phức tạp chỉ được phép ở các bộ phận thay được, tức là các collector.
- Nếu phiên bản hiện tại còn hàng đợi Redis, đó là đồ phụ. Mỗi lần thu thập là một file bất biến; một lần chạy hỏng chỉ là một file bị thiếu, và lần chạy sau bù vào.

Naught cũng là người đọc tự nhiên của biên bản do Helvilette ghi ra. Và bài nghiệm thu đẹp nhất cho Kallisto là chĩa Naught vào danh sách phụ thuộc của nó.

## 6. Thử súng

### 6.1. Hai lỗi đánh trượt

Một công cụ bị loại khỏi hệ sinh thái ngay khi mắc một trong hai lỗi sau, dù chỉ một lần:

1. **Tự cho mình khôn.** Làm việc sau lưng người vận hành và sinh ra tác dụng phụ.
2. **Bắt người dùng xây nhà cho nó.** Đòi Kubernetes, đòi một chuỗi phụ thuộc dài, đòi cài thêm runtime hay gói hệ điều hành, hoặc đã đóng container rồi vẫn phải vá lúc chạy mới lên được. Cài là phải chạy.

### 6.2. Ba bài thử

| Bài thử | Nội dung | Điều kiện đạt |
| --- | --- | --- |
| Bài thi trong bùn | Chạy công cụ trong điều kiện bị phá có chủ đích. Ví dụ với Helvilette: 50 máy liên tục bị sửa ngẫu nhiên `sshd_config`, xoá user, hạ phiên bản gói, tắt service | Phát hiện và kéo mọi máy về đúng khai báo, không lỗi nào |
| Concorde | 74 phút tải rất cao, có một bản vá nóng được đẩy lên giữa chừng | Không lỗi, không chết, không chập chờn, giữ nhịp suốt thời gian chạy |
| Homelab | Tác giả cài công cụ lên hạ tầng của chính mình | Tác giả dám để nó chạy |

Bài thi phải khớp với nhiệm vụ của cỗ máy. Dựng một cụm k3s nhiều máy là bài thi của khẩu súng đột kích, không phải của khẩu súng tuần tra.

Bài thứ ba khó nhất về mặt tâm lý, và cũng là thước đo trung thực nhất. Một công cụ mà chính tác giả không dám chạy thì chưa thể cấp phát cho ai.

## 7. Những gì không mượn từ Liên Xô

Học thuyết mượn phương pháp, không mượn toàn bộ di sản. Có ba điểm tách ra có chủ đích.

1. **Không hy sinh người dùng.** Kỹ thuật Liên Xô từng chấp nhận để kíp lái xe tăng ngồi chật chội, phi công khó quan sát. Học thuyết này đảo ngược điều đó: được phép xấu, không được phép làm khổ. Nó cũng loại bỏ **sự đơn giản chiều chuộng**: loại đơn giản làm đẹp mắt người mua nhưng đẩy độ phức tạp thật sang người đang nằm dưới gầm xe. Đây cũng là căn bệnh của nhiều nền tảng dành cho lập trình viên, vốn giấu độ phức tạp khỏi dev bằng cách dồn nó lên người vận hành.
2. **Không giữ bí mật.** Tài liệu là một bộ phận của súng, và mã nguồn để mở cho ai cũng tháo ra xem được.
3. **Phần mềm thì mục.** Một khẩu AK để trong kho ba mươi năm, lau dầu là bắn được. Phần mềm thì mục ngay cả khi không ai động vào: thư viện bị bỏ rơi, lỗ hổng mới xuất hiện, thẻ image bị dời đi. Đây là chỗ phép so sánh gãy, và cũng là lý do hệ sinh thái cần một kẻ quan sát như Naught.

## 8. Kết luận

Học thuyết này đề xuất một cách thiết kế phần mềm hạ tầng xuất phát từ người vận hành trong khoảnh khắc tệ nhất của họ. Nó mượn từ các phòng thiết kế vũ khí Liên Xô một phương pháp: xác định nhiệm vụ trước, làm cỗ máy đơn giản nhất đáp ứng được nhiệm vụ, và đặt độ phức tạp vào chỗ bảo trì được.

Ba ca thực chiến cho thấy học thuyết vừa dùng để đánh giá, vừa dùng để sửa thiết kế. Kallisto đạt chuẩn về thiết kế. Helvilette được tách thành một agent hoàn chỉnh và một hệ dẫn đường tuỳ chọn. Naught được chỉ ra một bộ phận đáng nghi ngờ.

### Câu hỏi mở

- Học thuyết cần được kiểm chứng bởi những người vận hành khác ngoài tác giả.
- "Vùng chạm không xác định" cần một cách đo thống nhất giữa các công cụ.
- Bản Helvilette điều phối nhiều máy cần một người dùng thật, với nhu cầu thật, trước khi được xây.

## Phụ lục A. Bảng tra nhanh

| Hình ảnh cơ khí | Nguyên lý | Ví dụ trong Naughtian |
| --- | --- | --- |
| Phương pháp của phòng thiết kế | Nhiệm vụ trước, hình dạng sau | Helvilette bỏ hình dạng sao chép Kubernetes |
| MiG-21 và radar mặt đất | Đặt độ phức tạp vào chỗ bảo trì được; máy bay vẫn bay khi mất liên lạc | Agent độc lập, Othela tuỳ chọn |
| Khoá nòng xoay | Mượn cơ cấu đã được chứng minh | `ansible-pull`, `arc-swap`, mô hình Borg trong Kalena |
| Băng đạn lắp lẫn | Giao diện chuẩn và bền nhất | API KV v2 của Kallisto; biên bản có số phiên bản của Helvilette |
| Nòng mạ chrome | Kiểm tra nghiêm ngặt ở cửa vào | Kiểm chữ ký commit; kiểm dữ liệu bên ngoài trong Naught |
| Tháo dã chiến | Hiểu được trong một ngày | Kallisto sau đợt cắt giảm |
| Ống lót tay | Thiết kế cho lúc người dùng quên | Dòng đánh dấu file, lời chào đăng nhập, `halt --reason` |
| Vách ngăn kín nước | Hỏng không lan dây chuyền | Dữ liệu dẫn xuất xoá và dựng lại được |
| Lọc dầu trong tầm tay | Được phép xấu, không được phép làm khổ | Giao diện dòng lệnh, tài liệu đầy đủ |

## Ghi chú nguồn và đóng góp

- Các chi tiết lịch sử về vũ khí Liên Xô trong bài là kiến thức phổ thông, **chưa được đối chiếu với nguồn**. Cần bổ sung trích dẫn trước khi công bố chính thức.
- Verma, A. và cộng sự. *Large-scale cluster management at Google with Borg*. EuroSys 2015.
- Beyer, B., Jones, C., Petoff, J., Murphy, N. R. (biên tập). *Site Reliability Engineering: How Google Runs Production Systems*. O'Reilly, 2016. Chương 1, mục "Change Management": "SRE has found that roughly 70% of outages are due to changes in a live system." https://sre.google/sre-book/introduction/
- Tài liệu của hệ sinh thái Naughtian: *The Letter from Naught*; white paper *Kuberina*; đề xuất *Naughtian Kalena*; README và ARCHITECTURE của Naught.
- **Đóng góp:** ý tưởng, chân dung người vận hành, các nguyên lý và tiêu chí đánh giá do Đinh Tấn Dũng đưa ra qua một buổi phỏng vấn. Claude (Anthropic) hỗ trợ đặt câu hỏi, cấu trúc và biên tập bản thảo.

## Bổ sung

1. **Sống sót kiểu trâu bò.** Phần mềm phải sống sót trong điều kiện vận hành và bảo trì thô bạo nhất có thể: bị tắt ngang, bị khởi động lại liên tục, bị bỏ quên lâu ngày, bị người không quen tay vận hành và bảo trì. Nó không được đòi hỏi sự chăm sóc khéo léo mới chạy tiếp được.
2. **Bộ test là bài thi trong bùn thường trực.** Bộ test phải mô phỏng những điều kiện cực đoan nhất mà phần mềm có thể gặp, và phần mềm vẫn phải vận hành được trong các điều kiện đó. Hình mẫu là SQLite: bộ test của nó đạt độ phủ theo chuẩn dành cho phần mềm hàng không, và cố ý giả lập những tình huống như hết bộ nhớ, lỗi ghi đĩa, mất điện giữa chừng.

## Bổ sung: đọc lại mục 3 qua lăng kính quan liêu hình thức

Mục 3 nên được đọc là **năm triệu chứng**, không phải năm tội. Phần mềm chỉ là triệu chứng. Căn bệnh là **quan liêu hình thức: thiết kế cho người duyệt, không cho người dùng.** Dev được chiều bằng trải nghiệm đẹp, người quản lý được chiều bằng dashboard và cảm giác kiểm soát, còn người vận hành nhận phần còn lại.

Năm triệu chứng của cùng một căn bệnh:

- **Làm cho có:** tính năng tồn tại để đánh dấu vào bảng so sánh lúc bán hàng, không phải để dùng.
- **Làm ngầm:** giấu độ phức tạp để bản demo trông đơn giản trong mắt người duyệt.
- **Chiếm máy:** ranh giới sản phẩm được vẽ theo lợi ích của nhà cung cấp, không theo công việc của người vận hành.
- **Câm khi hỏng:** thông báo lỗi được làm cho "đỡ đáng sợ" thay vì làm cho đúng.
- **Nuôi ảo tưởng cho cấp trên:** dashboard được thiết kế để người quản lý thấy mình đang kiểm soát mọi thứ.

### Nghịch lý Liên Xô

Liên Xô là một trong những bộ máy quan liêu nặng nề nhất lịch sử, nhưng vũ khí của họ thoát được căn bệnh này. Lý do là chiến trường là cuộc kiểm toán không ai làm giả được. Súng kẹt trong bùn thì báo cáo đẹp cỡ nào cũng vô ích, và thông tin đó quay ngược về phòng thiết kế.

Phần mềm cũng có chiến trường: sự cố lúc 2 giờ sáng. Nhưng phản hồi từ trận đó **không về tới người thiết kế**, vì người vận hành đã hứng hết: bị đổ lỗi, bị kỷ luật, rồi lặng lẽ chắp vá. Bộ máy quan liêu tồn tại được vì người vận hành làm vai tấm đệm.

Học thuyết này, xét cho cùng, là cách **nối lại đường phản hồi từ chiến trường về bàn vẽ**: bài thi trong bùn, bộ test mô phỏng điều kiện cực đoan kiểu SQLite, và quy tắc người viết phải dám chạy công cụ trên hạ tầng của chính mình.

### Ghi chép: NGINX, khẩu AK của hạ tầng hiện đại

- **Người lính tự rèn súng:** Igor Sysoev là sysadmin ở Rambler, viết NGINX vì Apache không giữ nổi hàng chục nghìn kết nối đồng thời; phát hành công khai năm 2004.
- **Ít chi tiết:** một master quản lý các worker; mỗi worker là một vòng lặp sự kiện đơn luồng dùng epoll hoặc kqueue của hệ điều hành.
- **Luật số 0:** `nginx -t` chạy thử cấu hình; khi reload, cấu hình sai bị từ chối và worker cũ vẫn chạy; lỗi báo đúng file và số dòng.
- **Vách ngăn kín nước:** một worker chết thì master dựng lại, các worker khác không bị ảnh hưởng.
- **Vá nóng:** thay file chạy sang phiên bản mới mà không rớt kết nối.
- **Chỗ trượt:** vài bẫy ngầm trong ngôn ngữ cấu hình (kế thừa `add_header`, `if`, dấu `/` cuối `proxy_pass`); và bản trả phí NGINX Plus giữ lại những thứ người vận hành cần lúc có sự cố, như kiểm tra sức khoẻ chủ động và đổi upstream không cần reload.
- **Số phận khẩu súng:** F5 mua NGINX năm 2019, Sysoev rời đi năm 2022; các bản tách Angie (2022) và freenginx (2024) ra đời. Giống AK được chép ở khắp nơi, thiết kế tốt sống lâu hơn người làm ra nó, và giấy phép mở đảm bảo không ai giữ được nó làm con tin.
- **Các biến thể hiện đại:** Kong (dựng trên OpenResty) và BunkerWeb là dòng AKM, AK-74 của NGINX. Chế độ hybrid của Kong lặp lại mô hình MiG-21 và radar mặt đất: data plane vẫn phục vụ bằng cấu hình nhận được lần cuối khi mất liên lạc với control plane.
