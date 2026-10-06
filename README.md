# micro-server · 后端服务仓

18 个后端服务（14 RPC + 2 BFF + iotingest + hello 模板）+ `tools/` 工具箱，go-zero 技术栈（goctl 生成 + 手写 Logic）。技术口径见《02-技术文档》（micro-docs 仓正本 / micro-deploy/docs 分发副本）。

## 目录骨架

```
micro-server/
├── go.work                    # workspace：tools（后续随服务增加 use）
├── services/                  # 18 个服务（goctl 生成骨架 + logic 实现 + model）
│   └── order/
│       ├── api/order.api      # 或 pb/order.proto —— 契约入库（唯一契约源）
│       ├── internal/
│       │   ├── config/        # 配置 Schema（${ENV} 占位注入敏感值）
│       │   ├── logic/         # 业务逻辑（唯一允许写业务的地方）
│       │   ├── handler/ | server/  # goctl 生成，禁止手改
│       │   ├── svc/           # ServiceContext 组装根
│       │   └── model/         # goctl model 生成 + custom 扩展
│       └── etc/order.yaml
├── tools/                     # migrate/seed/envcheck/dodcheck/apitypes/mqinit/devicesim/reconcile/newsvc/keygen/configpush
└── docs/swagger/              # goctl api swagger 生成物（入库，CI diff 阻断漂移）
```

## module 命名约定（S0-01）

- 服务：一服务一 module，命名 `micro-server/services/<svc>`（如 `micro-server/services/order`）
- 工具箱：`micro-server/tools`（每个工具一个 package main 子目录）
- 公共库：`micro-common`（独立仓；本地经开发机根 go.work 解析，S1 起 semver tag 发布）
- 本仓根 `go.work` 当前只含 `./tools`；新增服务后 `go work use ./services/<svc>` 加入

## 契约先行（S0-05，最重要的流程纪律）

**改任何接口，先改契约文件（`.api` / `.proto`），评审合并后再写实现。**

- swagger / apitypes 都以契约为唯一源：CI `swagger-diff` 作业用 `goctl api swagger` 重生成并 diff 阻断（ADR-20，goctl ≥ 1.8.4 内置 swagger）；apitypes 生成 TS 类型到 micro-web/packages/shared/types
- 生成物不入库手工修改（重新生成会覆盖）；确需定制模板时，模板与 goctl 版本一起锁定
- **每次生成后的固定流水线**（Makefile 与 CI 同款）：

```
go mod tidy → import 路径与 module 校验 → go build ./... → go vet → golangci-lint → goctl api swagger 重生成
```

- goctl 命令速查（全 `--style go_zero`）：

```bash
goctl api go   -api services/order/api/order.api -dir services/order --style go_zero      # REST 骨架
goctl rpc protoc services/order/pb/order.proto --go_out=. --go-grpc_out=. --zrpc_out=services/order --style go_zero
goctl model mysql ddl -src services/order/migrations/0001_init.up.sql -dir services/order/internal/model -cache --style go_zero
goctl api validate -api services/order/api/order.api                                    # 契约静态检查（CI）
goctl api swagger  --api services/admin-bff/api/entry.api --dir docs/swagger --filename admin-bff
goctl docker --go 1.26 --port 8090                                                      # Dockerfile 基线
```

## 服务与端口（02 §5.2 定稿，先登记后取号 E12）

| 服务 | RPC/HTTP | Metrics | 服务 | RPC/HTTP | Metrics |
|---|---|---|---|---|---|
| identity | 8081 | 9101 | order | 8090 | 9110 |
| party | 8082 | 9102 | station | 8091 | 9111 |
| catalog | 8083 | 9103 | finance | 8092 | 9112 |
| inventory | 8084 | 9104 | contract | 8093 | 9113 |
| device | 8085 | 9105 | ops | 8094 | 9114 |
| file | 8086 | 9106 | analytics | 8095 | 9115 |
| audit | 8087 | 9107 | hello（模板） | 8888 | 9117 |
| notification | 8089 | 9109 | admin-bff | 8889 | 9118 |
| iotingest | —（OCPP 8182） | 9120 | mobile-bff | 8890 | 9119 |

> 预留不分配：8088/9108、8096/9116。新服务先在此表与 02 §5.2 登记再取号。

## CI（S0-04）

`ci.yml`：build（workspace `go build micro-server/...`）→ test（mysql:8 + redis:7 service 容器，`MICRO_TEST_*` 直连）→ golangci-lint v2 → **swagger-diff** → tenantaudit 占位（S1-10 自动启用）→ dodcheck 占位（S3 自动启用）；
`images.yml`（仅 main）：docker-bake → GHCR → trivy 扫描（`docker-bake.hcl` 随首个服务 Dockerfile 落地，S3 起）。

## 常用命令

```bash
make build        # go build micro-server/...
make test         # go test ./...
make lint         # golangci-lint v2
make validate     # goctl api validate 全部 .api
make swagger      # goctl api swagger 重生成（CI 同款）
make migrate-up svc=order    # tools/migrate（S2-05 落地）
make restart svc=order       # tools/restart-svc.ps1（S2-05 落地）
```

## 提交与评审约定（S0-03）

- 中文 conventional commits + 任务号：`feat(order): S5-03 支付超时自动取消`；trunk-based（main + 短命分支）
- PR 走 DoD 自查 6 项模板；CODEOWNERS 管契约目录与 identity 等关键目录
- 支付 / 库存 / IoT 三模块双人熟悉制；跨仓库改动当天提交并推送
