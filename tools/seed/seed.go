// seed · 种子数据（S2-05）：default 租户 / 平台管理员 / 管理员角色
//
// 密码纪律（S2-04 口径）：
//   - 管理员密码只读环境变量 MICRO_ADMIN_PW（compose/dev/.env 也会被 devcfg 回读）；
//   - 未设置或强度不足（<8 位）→ 强告警并拒绝执行（绝不落默认弱密码）。
//
// 前置：identity 库表由 S3-01 迁移建立；本工具先做就绪校验，缺表时明确指向迁移命令。
// 幂等：按业务唯一键存在即更新，可反复执行。
// 注意：列清单按 S3-01 表设计书写；S3-01 若调整列，请同步本工具。
//
// 用法：go run ./tools/seed [-db micro_identity]
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/zxiaosi-micro/micro-common/crypto"

	"micro-server/tools/internal/devcfg"
)

// 表清单与 S3-01 identity_db 设计对齐；缺任一即视为迁移未跑。
var requiredTables = []string{"tenant", "user", "role", "user_role", "menu", "role_menu"}

// 种子数据用确定性 ID（联调/测试可依赖）。
const (
	seedTenantID = int64(9000000000000000001)
	seedAdminUID = int64(9000000000000000002)
	seedRoleID   = int64(9000000000000000003)
)

func main() {
	dbFlag := flag.String("db", "micro_identity", "identity 库名")
	flag.Parse()

	// ---- 1) 密码纪律：MICRO_ADMIN_PW 强校验（未设置强告警拒绝）----
	adminPW := devcfg.Get("MICRO_ADMIN_PW", "")
	if adminPW == "" {
		fmt.Fprintln(os.Stderr, "seed: [强告警] 环境变量 MICRO_ADMIN_PW 未设置——拒绝生成弱默认密码的管理员。")
		fmt.Fprintln(os.Stderr, "  设置后重跑：export MICRO_ADMIN_PW=<强密码>（或写入 compose/dev/.env）")
		os.Exit(1)
	}
	if len(adminPW) < 8 {
		fmt.Fprintf(os.Stderr, "seed: [强告警] MICRO_ADMIN_PW 长度 %d < 8——拒绝执行（避免弱种子密码）。\n", len(adminPW))
		os.Exit(1)
	}

	// ---- 2) 连接 ----
	host := devcfg.Get("MICRO_DEV_MYSQL_HOST", "127.0.0.1:23306")
	user := devcfg.Get("MICRO_DEV_MYSQL_APP_USER", "micro_app")
	pw := devcfg.Get("MICRO_DEV_MYSQL_APP_PW", "")
	if pw == "" {
		fatal(errors.New("MICRO_DEV_MYSQL_APP_PW 未设置（先跑 envcheck 排查环境）"))
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&timeout=5s", user, pw, host, *dbFlag)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatal(fmt.Errorf("连库 %s 失败: %w（先跑 go run ./tools/migrate -svc identity up）", *dbFlag, err))
	}

	// ---- 3) 表就绪校验（缺表明确指向 S3-01 迁移）----
	for _, t := range requiredTables {
		var n int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?`,
			*dbFlag, t).Scan(&n); err != nil || n == 0 {
			fatal(fmt.Errorf("表 %s.%s 不存在——先执行：go run ./tools/migrate -svc identity up（S3-01）", *dbFlag, t))
		}
	}

	// ---- 4) 数据密钥（mobile 加密列用；S1-05 crypto）----
	keyProvider, err := crypto.EnvKeyProviderFromEnv()
	if err != nil {
		fatal(fmt.Errorf("MICRO_DATA_KEYS 未就绪（keygen 产出后写入 .env）: %w", err))
	}
	enc, err := crypto.NewEncryptor(keyProvider)
	if err != nil {
		fatal(err)
	}
	mobileCipher, err := enc.Encrypt([]byte("13800000000"))
	if err != nil {
		fatal(fmt.Errorf("mobile 加密失败: %w", err))
	}
	mobileHash := hmacIndex(devcfg.Get("MICRO_HASH_KEY", "micro-dev-index-key"), "13800000000")

	// ---- 5) 幂等种子 ----
	now := time.Now()
	mustExec(db,
		`INSERT INTO tenant (tenant_id, tenant_code, name, plan, quota, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE name=VALUES(name), updated_at=VALUES(updated_at)`,
		seedTenantID, "default", "默认租户", "STANDARD", `{}`, now, now)
	fmt.Println("tenant     default 租户就绪")

	types, _ := json.Marshal([]string{"ADMIN_WEB", "OPS_APP"})
	mustExec(db,
		`INSERT INTO user (user_id, org_id, mobile, mobile_hash, email, password_hash, types, status,
		                   tenant_id, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash), mobile=VALUES(mobile),
		                         mobile_hash=VALUES(mobile_hash), updated_at=VALUES(updated_at)`,
		seedAdminUID, sql.NullInt64{}, mobileCipher, mobileHash, "admin@example.local",
		argon2Hash(adminPW), string(types), 1, seedTenantID, now, now)
	fmt.Printf("user       平台管理员就绪（uid=%d，argon2id，密码=MICRO_ADMIN_PW）\n", seedAdminUID)

	mustExec(db,
		`INSERT INTO role (role_id, code, name, data_scope, tenant_id, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE name=VALUES(name), updated_at=VALUES(updated_at)`,
		seedRoleID, "admin", "平台管理员", `{"type":"ALL"}`, seedTenantID, now, now)
	fmt.Println("role       admin 角色就绪（data_scope=ALL）")

	mustExec(db,
		`INSERT INTO user_role (user_id, role_id, tenant_id, created_at)
		 VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE created_at=VALUES(created_at)`,
		seedAdminUID, seedRoleID, seedTenantID, now)
	fmt.Println("user_role  管理员绑定 admin 角色")

	fmt.Println("menu/role_menu：业务菜单随 S3-03 菜单数据落地后补种（不预造业务数据）")
	fmt.Println("== seed 完成 ==")
}

// ---- 依赖薄封装（失败即退出，无静默）----

func mustExec(db *sql.DB, q string, args ...any) {
	if _, err := db.Exec(q, args...); err != nil {
		fatal(fmt.Errorf("seed 写入失败: %w（S3-01 表结构若调整请同步本工具）", err))
	}
}

func argon2Hash(pw string) string {
	h, err := crypto.HashPassword(pw)
	if err != nil {
		fatal(fmt.Errorf("argon2id 哈希失败: %w", err))
	}
	return h
}

// hmacIndex 取 mobile_hash：HMAC-SHA256 + 索引键（S3-01 口径：加密列配 hash 列做唯一索引；
// 键经 MICRO_HASH_KEY 注入，S3-01 落地后统一改用 identity 服务的实现）。
func hmacIndex(key, plain string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(plain))
	return hex.EncodeToString(mac.Sum(nil))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "seed: %v\n", err)
	os.Exit(1)
}
