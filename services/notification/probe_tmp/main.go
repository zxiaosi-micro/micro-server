package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, _ := sql.Open("mysql", os.Getenv("DSN"))
	defer db.Close()
	_, err := db.Exec("insert into `user_notify_setting` (`setting_id`, `user_id`, `template_code`, `enabled`, `quiet_hours`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?) on duplicate key update `enabled` = values(`enabled`), `quiet_hours` = values(`quiet_hours`), `updated_by` = values(`updated_by`), `updated_at` = current_timestamp",
		123, 900000000000000002, "smoke.test", 1, "x", 900000000000000001, sql.NullInt64{}, sql.NullInt64{})
	fmt.Println("upsert err:", err)
}
