// migrate · 建库 + 成对迁移（S2-05）
//
// 用法（v4/03 S3-05 示范口径）：
//
//	go run ./tools/migrate -svc identity up           # 自动建库 micro_identity + 应用全部 up 迁移
//	go run ./tools/migrate -svc identity down         # 回滚最近 1 个迁移
//	go run ./tools/migrate -svc identity down -all    # 回滚全部
//	go run ./tools/migrate -svc identity down -steps 2
//
// 约定：
//   - 库名 = micro_<svc>（S2-01 授权口径 micro_%，env.md §2）；应用账号通配授权可直接建库。
//   - 迁移目录默认 services/<svc>/migrations，up/down 必须成对（CI 静态检查成对，v4/02 §6.1）。
//   - DSN 带多语句执行（一个迁移文件可含多条 SQL，goctl model 以迁移 SQL 为模型源，ADR-08）。
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"micro-server/tools/internal/devcfg"
)

func main() {
	// 子命令（up/down）允许出现在任意位置：先摘出，再让 flag 包解析剩余参数
	sub := ""
	args := os.Args[1:]
	for i, a := range args {
		if a == "up" || a == "down" {
			sub = a
			args = slices.Delete(args, i, i+1)
			break
		}
	}

	svc := flag.String("svc", "", "服务名（必填，如 identity）")
	dir := flag.String("dir", "", "迁移目录（默认 services/<svc>/migrations，相对 micro-server 根）")
	dbFlag := flag.String("db", "", "库名（默认 micro_<svc>）")
	all := flag.Bool("all", false, "down 时回滚全部")
	steps := flag.Int("steps", 0, "down 时回滚 N 个迁移")
	_ = flag.CommandLine.Parse(args)

	if *svc == "" || (sub != "up" && sub != "down") {
		fmt.Fprintln(os.Stderr, "用法：go run ./tools/migrate -svc <name> up|down [-dir ...] [-all|-steps N]")
		os.Exit(2)
	}

	host := devcfg.Get("MICRO_DEV_MYSQL_HOST", "127.0.0.1:23306")
	user := devcfg.Get("MICRO_DEV_MYSQL_APP_USER", "micro_app")
	pw := devcfg.Get("MICRO_DEV_MYSQL_APP_PW", "")
	if pw == "" {
		fmt.Fprintln(os.Stderr, "migrate: MICRO_DEV_MYSQL_APP_PW 未设置（compose/dev/.env）")
		os.Exit(1)
	}
	dbName := *dbFlag
	if dbName == "" {
		dbName = "micro_" + *svc
	}

	// ---- 1) 建库（应用账号对 micro_% 有 db 级 CREATE，可自行建库）----
	serverDSN := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=5s", user, pw, host)
	sdb, err := sql.Open("mysql", serverDSN)
	if err != nil {
		fatal(err)
	}
	if _, err := sdb.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci", dbName)); err != nil {
		fatal(fmt.Errorf("建库 %s: %w", dbName, err))
	}
	_ = sdb.Close()
	fmt.Printf("库 %s 就绪（micro_<svc> 约定，env.md §2）\n", dbName)

	// ---- 2) 定位迁移目录 ----
	migDir := *dir
	if migDir == "" {
		migDir = fmt.Sprintf("services/%s/migrations", *svc)
	}
	if _, err := os.Stat(migDir); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: 迁移目录不存在：%s\n（S3-01 落地 <svc> 的 000001_init.up/down.sql 后可用；可先 -dir 指定其它目录）\n", migDir)
		os.Exit(1)
	}
	fsys := os.DirFS(migDir)
	src, err := iofs.New(fsys, ".")
	if err != nil {
		fatal(fmt.Errorf("读迁移目录 %s: %w", migDir, err))
	}

	// ---- 3) golang-migrate ----
	dbDSN := fmt.Sprintf("%s:%s@tcp(%s)/%s?multiStatements=true&parseTime=true&timeout=5s", user, pw, host, dbName)
	m, err := migrate.NewWithSourceInstance("iofs", src, "mysql://"+dbDSN)
	if err != nil {
		fatal(err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			fmt.Printf("migrate: close: source=%v db=%v\n", srcErr, dbErr)
		}
	}()

	switch sub {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			fatal(fmt.Errorf("up: %w", err))
		} else if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("迁移已是最新（无变更）")
		} else {
			fmt.Println("up 迁移应用完成")
		}
	case "down":
		if !*all && *steps <= 0 {
			fmt.Fprintln(os.Stderr, "migrate: down 需 -steps N 或 -all（防止误全量回滚）")
			os.Exit(2)
		}
		if *all {
			// 逐版本回滚；NilVersion（从未有迁移/已回滚完）直接完成
			for {
				if _, _, verr := m.Version(); errors.Is(verr, migrate.ErrNilVersion) {
					break
				} else if verr != nil {
					fatal(fmt.Errorf("down -all: 读当前版本: %w", verr))
				}
				if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
					fatal(fmt.Errorf("down -all: %w", err))
				}
			}
			fmt.Println("已回滚全部迁移（当前无版本记录）")
		} else {
			if err := m.Steps(-*steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				fatal(fmt.Errorf("down -steps %d: %w", *steps, err))
			}
			fmt.Printf("已回滚 %d 个迁移\n", *steps)
		}
	}

	// dirty 状态明确报出（半执行状态必须人工处理，不许静默）
	if _, dirty, verr := m.Version(); verr == nil && dirty {
		fmt.Fprintln(os.Stderr, "migrate: 当前处于 DIRTY 状态——检查失败原因后强制 version 或修复数据，禁止直接重跑")
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
	os.Exit(1)
}
