# tools · 工具箱

module `micro-server/tools`，每个工具一个 package main 子目录，自 S2-05 起逐个落地：

| 工具 | 职责 | 落地阶段 |
|---|---|---|
| migrate | 建库 + golang-migrate 成对迁移（-svc up/down） | S2-05 |
| seed | 种子数据（default 租户/管理员/角色菜单，密码读 MICRO_ADMIN_PW） | S2-05 |
| envcheck | MySQL/Redis/etcd/TDengine/Kafka 五项连通 x/5 | S2-03 |
| keygen | 密钥三件套（RS256/Ed25519/数据密钥）→ deploy/conf/keys（gitignore） | S2-04 |
| newsvc | 新服务脚手架（封装 goctl + 端口登记校验） | S2-05 |
| dodcheck | 上线 DoD 静态把关（指标/Telemetry/健康检查/runbook 五节，0 FAIL 可合并） | S3 |
| tenantaudit | 租户表 model SQL 静态对账（tenant_id 条件/列，S1-10 配套） | S1-10 |
| apitypes | .api → TS 类型 → micro-web/packages/shared/types（CI diff 阻断） | S3 |
| mqinit | Kafka topic 预创建（下划线命名） | S2 |
| devicesim | 设备模拟器（2000 台协议标准报文） | S6 |
| reconcile | 库存/资金日结对账 | S5 |
| configpush | 配置正本校验 + 推送 etcd（/micro/config/...） | S2-06 |
