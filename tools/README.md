# tools · 工具箱

module `micro-server/tools`，每个工具一个 package main 子目录。凭据读取统一走
`internal/devcfg`：进程环境变量 > `../micro-deploy/compose/dev/.env` 回读（go run 零配置可用）。

| 工具 | 职责 | 落地阶段 | 状态 |
|---|---|---|---|
| migrate | 建库（micro_<svc>）+ golang-migrate 成对迁移（-svc up/down） | S2-05 | ✅ 2026-10-07 |
| seed | 种子数据（default 租户/管理员/角色；密码读 MICRO_ADMIN_PW，未设置强告警拒绝） | S2-05 | ✅ 2026-10-07（INSERT 随 S3-01 表落地启用） |
| envcheck | MySQL/Redis/etcd/TDengine(带认证 E7)/Kafka(E3) 五项连通 x/5 | S2-03 | ✅ 2026-10-07 |
| keygen | 密钥三件套（RS256+kid/Ed25519/AES 数据密钥）→ ../micro-deploy/deploy/conf/keys | S2-04 | ✅ 2026-10-07 |
| newsvc.sh | 新服务脚手架（封装 goctl + 端口登记校验 port_registry.txt，E12） | S2-05 | ✅ 2026-10-07 |
| configpush | 配置正本校验 + 推送 etcd（/micro/config/...）+ `-demo` listener 演示 | S2-06 | ✅ 2026-10-07 |
| restart-svc.ps1 | 重编译 + 拉起单个服务 + 日志重定向 logs/ | S2-05 | ✅ 2026-10-07 |
| port_registry.txt | 服务端口取号源（与 v4/02 §5.2 同源，E12） | S2-05 | ✅ 2026-10-07 |
| dodcheck | 上线 DoD 静态把关（指标/Telemetry/健康检查/runbook 五节，0 FAIL 可合并） | S3 | ⬜ |
| tenantaudit | 租户表 model SQL 静态对账（tenant_id 条件/列，S1-10 配套） | S3/S4 | ⬜（S1-10 已交付规则口径） |
| apitypes | .api → TS 类型 → micro-web/packages/shared/types（CI diff 阻断） | S3 | ⬜ |
| mqinit | Kafka topic 预创建（下划线命名，E2） | S5 | ⬜ |
| devicesim | 设备模拟器（2000 台协议标准报文） | S6 | ⬜ |
| reconcile | 库存/资金日结对账 | S5 | ⬜ |

## 常用命令

```bash
go run ./tools/envcheck                          # 中间件体检（5/5 才继续）
go run ./tools/migrate -svc identity up          # 建库 micro_identity + 迁移
go run ./tools/seed                              # 种子（MICRO_ADMIN_PW 必须设置）
go run ./tools/keygen                            # 密钥三件套 → micro-deploy/deploy/conf/keys
bash tools/newsvc.sh demo rpc                    # 新服务骨架（端口必须已登记）
go run ./tools/configpush -validate              # 配置正本校验（CI）
go run ./tools/configpush                        # 推送 etcd
go run ./tools/configpush -demo                  # configcenter listener 全链路演示
pwsh -File tools/restart-svc.ps1 order           # 重编译+拉起+日志重定向
```
