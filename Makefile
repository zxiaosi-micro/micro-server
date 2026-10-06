# micro-server 常用命令（S0-02）
# 用法：make <target>；Windows 本地若无 make，按各 target 直接执行等价命令（README 有速查）
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

.DEFAULT_GOAL := help
.PHONY: build test vet lint tidy fmt gen validate swagger migrate-up migrate-down restart clean help

## build: 编译 workspace 全部模块（CI 同款）
build:
	go build micro-server/...

## test: 全量单测（集成测试三级解析：MICRO_TEST_* → 本机 → testcontainers → Skip）
# workspace 根不是 module，./... 会报错——统一用 module 路径模式
test:
	go test micro-server/...

## vet: go vet
vet:
	go vet micro-server/...

## lint: golangci-lint v2，逐模块运行（workspace 根无 go.mod，必须进模块目录跑）
lint:
	@for d in $$(git ls-files '*/go.mod' | xargs -n1 dirname | sort -u); do \
		echo "== lint $$d"; (cd "$$d" && golangci-lint run ./...); \
	done

## tidy: 逐模块 go mod tidy（workspace 下需进入各模块执行）
tidy:
	@for d in $$(git ls-files '*/go.mod' 'go.mod' | grep -v '^node_modules' | xargs -n1 dirname | sort -u); do \
		echo "== tidy $$d"; (cd "$$d" && go mod tidy); \
	done

## fmt: gofmt 全仓
fmt:
	gofmt -l -w .

## gen: goctl 代码生成（改契约后必跑；命令速查见 README「契约先行」）
gen:
	@echo "改契约后按 README 速查执行 goctl 生成（全 --style go_zero）；"
	@echo "生成后固定流水线：tidy → import 校验 → build → vet → lint → make swagger"
	@echo "新服务骨架：S2-05 落地 tools/newsvc 后统一走 go run ./tools/newsvc <name> rpc|api"

## validate: 契约静态检查（.api，CI 同款）
validate:
	@apis=$$(git ls-files 'services/*/*.api' 'services/*/api/*.api'); \
	[ -n "$$apis" ] || { echo "尚无 .api 契约"; exit 0; }; \
	for f in $$apis; do echo "== validate $$f"; goctl api validate -api "$$f"; done

## swagger: 重生成 OpenAPI 文档到 docs/swagger（CI swagger-diff 同款命令）
swagger:
	@entries=$$(git ls-files 'services/*/api/entry.api'); \
	[ -n "$$entries" ] || { echo "尚无 entry.api（入口契约）"; exit 0; }; \
	mkdir -p docs/swagger; \
	for f in $$entries; do \
		svc=$$(echo "$$f" | cut -d/ -f2); \
		echo "== swagger $$svc"; goctl api swagger --api "$$f" --dir docs/swagger --filename "$$svc"; \
	done

## migrate-up: 建库并应用迁移；用法 make migrate-up svc=order（tools/migrate 于 S2-05 落地）
migrate-up:
	@test -n "$(svc)" || { echo "用法：make migrate-up svc=<服务名>"; exit 1; }
	@test -d tools/migrate || { echo "tools/migrate 未落地（S2-05）"; exit 1; }
	go run ./tools/migrate -svc $(svc) up

## migrate-down: 回滚迁移；用法 make migrate-down svc=order
migrate-down:
	@test -n "$(svc)" || { echo "用法：make migrate-down svc=<服务名>"; exit 1; }
	@test -d tools/migrate || { echo "tools/migrate 未落地（S2-05）"; exit 1; }
	go run ./tools/migrate -svc $(svc) down

## restart: 重编译并拉起单个服务；用法 make restart svc=order（tools/restart-svc.ps1 于 S2-05 落地）
restart:
	@test -n "$(svc)" || { echo "用法：make restart svc=<服务名>"; exit 1; }
	@test -f tools/restart-svc.ps1 || { echo "tools/restart-svc.ps1 未落地（S2-05）"; exit 1; }
	pwsh -File tools/restart-svc.ps1 $(svc)

## clean: 清理本地构建产物
clean:
	@find . -name '*.exe' -delete 2>/dev/null || true

## help: 目标清单
help:
	@echo "build / test / vet / lint / tidy / fmt / gen / validate / swagger"
	@echo "migrate-up svc=x / migrate-down svc=x / restart svc=x / clean"
