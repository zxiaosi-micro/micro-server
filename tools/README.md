# tools · 工具箱

module `micro-server/tools`，每个工具一个 package main 子目录。凭据读取统一走
`internal/devcfg`：进程环境变量 > `../micro-deploy/compose/dev/.env` 回读（go run 零配置可用）。

## 基建与生成

| 工具 | 职责 | 落地阶段 | 状态 |
|---|---|---|---|
| migrate | 建库（micro_<svc>）+ golang-migrate 成对迁移（-svc up/down） | S2-05 | ✅ 2026-10-07 |
| seed | 种子数据（default 租户/管理员/角色；密码读 MICRO_ADMIN_PW，未设置强告警拒绝） | S2-05 | ✅ 2026-10-07（INSERT 随 S3-01 表落地启用） |
| envcheck | MySQL/Redis/etcd/TDengine(带认证 E7)/Kafka(E3) 五项连通 x/5 | S2-03 | ✅ 2026-10-07 |
| keygen | 密钥三件套（RS256+kid/Ed25519/AES 数据密钥）→ ../micro-deploy/deploy/conf/keys | S2-04 | ✅ 2026-10-07 |
| newsvc.sh | 新服务脚手架（封装 goctl + 端口登记校验 port_registry.txt，E12） | S2-05 | ✅ 2026-10-07 |
| configpush | 配置正本校验 + 推送 etcd（/micro/config/...）+ `-demo` listener 演示 + `-diff` 正本与 etcd 双向一致性核对（S11-05） | S2-06 | ✅ 2026-10-07（-diff 2026-10-13） |
| apitypes | .api → TS 类型 → micro-web/packages/shared/types（CI diff 阻断） | S3 | ✅ 2026-10-08 |
| mqinit | Kafka topic 预创建（E2 先建后用，与 eventbus/topics.go 同步，现 23 topic） | S5 | ✅ 2026-10-08 |
| devenv.sh | dev 环境变量统一注入（DSN/Redis/数据密钥/HMAC/JWT/EMQX 双账号/TDengine/OTA 密钥） | S4-05 | ✅ 2026-10-08 |
| restart-svc.ps1 | 重编译 + 拉起单个服务 + 日志重定向 logs/ | S2-05 | ✅ 2026-10-07 |
| port_registry.txt | 服务端口取号源（与 v4/02 §5.2 同源，E12） | S2-05 | ✅ 2026-10-07 |
| smoke/ | 多端联调启停器（smoke-start-p5.sh 11 服务 / start-one.ps1 分离拉起 / stop-one.ps1 按端口硬杀） | S9 | ✅ 2026-10-11（exe 需预构建到 Temp/micro-smoke/，脚本不自动构建） |

## 静态把关（CI/Makefile 门槛）

| 工具 | 职责 | 落地阶段 | 状态 |
|---|---|---|---|
| tenantaudit | 多租户 SQL 静态对账（AST 轻量求值 + Sprintf 内联；INSERT 列/WHERE 条件/迁移 DDL 三规则；`tenantaudit:ignore` 行豁免须给原因；0 FAIL 门槛） | S11-01 | ✅ 2026-10-13（CI 目录出现即启用 + make tenantaudit） |
| secretcheck | 密钥出库核查（git 实证四规则：危险文件跟踪/私钥块/.env 忽略/keygen 目录忽略；全 workspace 扫描） | S11-05 | ✅ 2026-10-13 |
| commoncheck | 公共约定静态检查 | S4 | ✅ |
| dodcheck | 上线 DoD 静态把关（指标/Telemetry/健康检查/runbook 五节，0 FAIL 可合并） | S3 | ⬜（CI 占位，runbook 五节结构已按约定执行） |

## 冒烟与验收（严禁并发执行，E1）

| 工具 | 职责 | 落地阶段 | 状态 |
|---|---|---|---|
| e2esmoke | S5 交易全链（导入 SN→入库→下单→支付→Saga→出库→签收→质保） | S5-05 | ✅ 2026-10-09 |
| chaosmoke | Saga 韧性冒烟（02 §9.1 前六项：补偿/幂等/丢响应/停靠可观测/重推/并发不超卖） | S5-05 | ✅ 2026-10-09 |
| paytimeout | 支付超时验收辅助（建单不付→等 order 扫描取消） | S5-05 | ✅ 2026-10-09 |
| docktest | 容器化编排冒烟（停靠→自动续推链路） | S5-05 | ✅ 2026-10-09 |
| iotsmoke | S6 IoT 全链（状态机/激活/影子/端到端 P95<5s/越限候选/指令下行） | S6 | ✅ 2026-10-11 |
| otasmoke | S6 OTA 冒烟（签名/篡改拒/状态机/灰度/回滚 8/8） | S6 | ✅ 2026-10-11 |
| devicesim | 设备协议标准报文模拟器（-n 2000 -rate 5 压测段；S11-02 修复 ack 等 token/stop 生效/Disconnect 优雅排空三处丢报缺陷；2000 台 72h 长测待驻留窗口） | S6 | ✅ 2026-10-10（丢报修复 2026-10-13） |
| ocppsim | OCPP 1.6J 模拟桩 | S6 | ✅ 2026-10-11 |
| p3smoke | S7 运维域冒烟（告警闭环/超时升级/SLA 升级/巡检转工单/订阅消息触达 15/15） | S7 | ✅ 2026-10-11 |
| p4smoke | S8 分析域冒烟（读模型/对账抓漏发/backfill 闭环/三格式导出 17/17） | S8 | ✅ 2026-10-11 |
| p4load | S8 导出负载验收（10 万行 <5s + 在线零影响） | S8 | ✅ 2026-10-11 |
| p4backfill | 读模型补投递（按业务库重放缺失事件；对账差异修复唯一手段，E10 禁裸删） | S8 | ✅ 2026-10-11 |
| p5smoke | S9 多端冒烟（MOCKWX 登录链/requireParty 隔离/互斥矩阵 11/11） | S9 | ✅ 2026-10-11 |
| s10smoke | S10 增值域冒烟（退役评级三分支/BI 白名单拒绝/ETL 幂等/返利快照+对账） | S10 | ✅ 2026-10-12 |
| pentest | S11 双租户隔离渗透 5 面 + 配额 429 全链（110429/RateLimit 头/恢复 200） | S11-01 | ✅ 2026-10-13 |
| p5perf | S11 NFR 性能复测（PRF-001/005/006；ramp-up 加压 + 压测客户端连接池调优） | S11-02 | ✅ 2026-10-13 |
| drill | S11 故障演练四场景（station 停机 Saga 卡建站/inventory 不超卖/analytics lag 积压+幂等/cron E16 逾期补跑；会停启四个服务，独立演练窗口执行） | S11-03 | ✅ 2026-10-13（脚本就绪；四场景实跑待演练窗口） |
| reconcile | 库存/资金日结对账（tools/reconcile 资金对账数据源 finance SumByChannelDate） | S5-05 | ✅ 2026-10-09 |
| kafkaprobe | 宿主机消费路径探针（E3：容器内 CLI 不可达 dev broker 时验证组消费可达性） | S5-05 | ✅ 2026-10-09 |

## 常用命令

```bash
go run ./tools/envcheck                          # 中间件体检（5/5 才继续）
go run ./tools/migrate -svc identity up          # 建库 micro_identity + 迁移
go run ./tools/seed                              # 种子（MICRO_ADMIN_PW 必须设置）
go run ./tools/keygen                            # 密钥三件套 → micro-deploy/deploy/conf/keys
bash tools/newsvc.sh demo rpc                    # 新服务骨架（端口必须已登记）
go run ./tools/configpush -validate              # 配置正本校验（CI）
go run ./tools/configpush                        # 推送 etcd
go run ./tools/configpush -diff                  # 正本 vs etcd 一致性核对（S11-05）
go run ./tools/configpush -demo                  # configcenter listener 全链路演示
pwsh -File tools/restart-svc.ps1 order           # 重编译+拉起+日志重定向
make tenantaudit                                 # 多租户 SQL 静态对账（0 FAIL 门槛）
```
