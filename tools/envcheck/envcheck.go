// envcheck · dev 中间件连通体检（S2-03）
//
// 中间件问题占新手环境故障 90%（v4/03）——跑一次知道差什么，而不是先写代码再排查。
// 五项：MySQL(23306) / Redis(26379) / etcd(22379) / TDengine(26041 带认证，E7) / Kafka(29092)。
// 凭据读取优先级见 internal/devcfg：进程环境变量 > compose/dev/.env > 端口默认值。
//
// 用法：go run ./tools/envcheck（退出码 0=5/5 全通，1=有失败）
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"

	"micro-server/tools/internal/devcfg"

	_ "github.com/go-sql-driver/mysql"
	kafkago "github.com/segmentio/kafka-go"
)

const timeout = 3 * time.Second

type check struct {
	name string
	addr string
	fn   func(ctx context.Context) (string, error)
}

func main() {
	cfg := devcfg.Env()
	if cfg.Path != "" {
		fmt.Printf("凭据来源：进程环境变量优先，回读 %s（%d 项）\n\n", cfg.Path, cfg.Count)
	}

	mysqlHost := devcfg.Get("MICRO_DEV_MYSQL_HOST", "127.0.0.1:23306")
	redisAddr := devcfg.Get("MICRO_DEV_REDIS_ADDR", "127.0.0.1:26379")
	etcdHosts := devcfg.Get("MICRO_DEV_ETCD_HOSTS", "127.0.0.1:22379")
	tdRest := devcfg.Get("MICRO_DEV_TDENGINE_REST", "http://127.0.0.1:26041")
	kafkaBrokers := devcfg.Get("MICRO_DEV_KAFKA_BROKERS", "127.0.0.1:29092")

	appUser := devcfg.Get("MICRO_DEV_MYSQL_APP_USER", "micro_app")
	appPW := devcfg.Get("MICRO_DEV_MYSQL_APP_PW", "")
	tdPW := devcfg.Get("MICRO_DEV_TDENGINE_PW", "")
	redisPW := devcfg.Get("MICRO_DEV_REDIS_PW", "")

	checks := []check{
		{"MySQL", mysqlHost, func(ctx context.Context) (string, error) {
			if appPW == "" {
				return "", fmt.Errorf("MICRO_DEV_MYSQL_APP_PW 未设置（compose/dev/.env）")
			}
			dsn := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=%s&readTimeout=%s&writeTimeout=%s",
				appUser, appPW, mysqlHost, timeout, timeout, timeout)
			db, err := sql.Open("mysql", dsn)
			if err != nil {
				return "", err
			}
			defer db.Close()
			var ver string
			if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&ver); err != nil {
				return "", fmt.Errorf("SELECT VERSION(): %w（账号 %s 是否已建？见 mysql/init/01-app-user.sh）", err, appUser)
			}
			return fmt.Sprintf("server %s（应用账号 %s）", ver, appUser), nil
		}},
		{"Redis", redisAddr, func(ctx context.Context) (string, error) {
			if redisPW == "" {
				return "", fmt.Errorf("MICRO_DEV_REDIS_PW 未设置（compose/dev/.env）")
			}
			cli := redis.NewClient(&redis.Options{Addr: redisAddr, Password: redisPW, DialTimeout: timeout})
			defer func() { _ = cli.Close() }()
			if err := cli.Ping(ctx).Err(); err != nil {
				return "", fmt.Errorf("PING: %w（DB0/1/3/4 划分见 v4/02 §6.3）", err)
			}
			return "PONG", nil
		}},
		{"etcd", etcdHosts, func(ctx context.Context) (string, error) {
			cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcdHosts}, DialTimeout: timeout})
			if err != nil {
				return "", err
			}
			defer cli.Close()
			key := fmt.Sprintf("/micro/envcheck/ping-%d", time.Now().UnixNano())
			if _, err := cli.Put(ctx, key, "pong"); err != nil {
				return "", fmt.Errorf("put: %w", err)
			}
			get, err := cli.Get(ctx, key)
			if err != nil || len(get.Kvs) == 0 {
				return "", fmt.Errorf("get: %w", err)
			}
			_, err = cli.Delete(ctx, key)
			if err != nil {
				return "", fmt.Errorf("delete: %w", err)
			}
			return "Put/Get/Delete 往返 OK（服务发现+configcenter 可用）", nil
		}},
		{"TDengine", tdRest, func(ctx context.Context) (string, error) {
			if tdPW == "" {
				return "", fmt.Errorf("MICRO_DEV_TDENGINE_PW 未设置（compose/dev/.env）")
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, tdRest+"/rest/sql",
				strings.NewReader("SELECT SERVER_VERSION()"))
			if err != nil {
				return "", err
			}
			// E7：TDengine REST 必须 Basic 认证——401 易被误判成"空结果"
			req.SetBasicAuth("root", tdPW)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
			if resp.StatusCode == http.StatusUnauthorized {
				return "", fmt.Errorf("401 Basic 认证失败（E7：REST 必须认证，检查 MICRO_DEV_TDENGINE_PW）")
			}
			var out struct {
				Code int      `json:"code"`
				Desc string   `json:"desc"`
				Data []string `json:"data"`
			}
			// data 可能是 [["3.3.6.13"]]，用二维兜底
			var out2 struct {
				Code int     `json:"code"`
				Data [][]any `json:"data"`
			}
			if err := json.Unmarshal(body, &out2); err == nil {
				out.Code = out2.Code
				for _, row := range out2.Data {
					if len(row) > 0 {
						out.Data = append(out.Data, fmt.Sprint(row[0]))
					}
				}
			}
			if out.Code != 0 {
				return "", fmt.Errorf("rest/sql code=%d desc=%s", out.Code, out.Desc)
			}
			ver := ""
			if len(out.Data) > 0 {
				ver = out.Data[0]
			}
			return fmt.Sprintf("server %s（Basic 认证 OK，E7）", ver), nil
		}},
		{"Kafka", kafkaBrokers, func(ctx context.Context) (string, error) {
			d := &kafkago.Dialer{Timeout: timeout}
			conn, err := d.DialContext(ctx, "tcp", kafkaBrokers)
			if err != nil {
				return "", fmt.Errorf("dial: %w（E3：advertised listener 必须 127.0.0.1:29092）", err)
			}
			defer conn.Close()
			// Brokers/ReadPartitions 都基于 Metadata 请求：连上但拿不到元数据 = advertised listener 配错（E3）
			brokerList, err := conn.Brokers()
			if err != nil {
				return "", fmt.Errorf("brokers: %w（能连上拿不到元数据 = advertised listener 配错，E3）", err)
			}
			parts, err := conn.ReadPartitions()
			if err != nil {
				return "", fmt.Errorf("ReadPartitions: %w（E3）", err)
			}
			topics := map[string]struct{}{}
			for _, pt := range parts {
				topics[pt.Topic] = struct{}{}
			}
			return fmt.Sprintf("brokers=%d topics=%d（KRaft 单机）", len(brokerList), len(topics)), nil
		}},
	}

	fmt.Println("== envcheck · dev 中间件连通体检 ==")
	pass := 0
	for i, c := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		detail, err := c.fn(ctx)
		cancel()
		mark, extra := "✗", ""
		if err == nil {
			mark, extra, pass = "✓", " "+detail, pass+1
		} else {
			extra = " " + err.Error()
		}
		fmt.Printf("[%d/5] %-9s %-24s %s%s\n", i+1, c.name, c.addr, mark, extra)
	}
	fmt.Printf("\n结果：%d/5 通过", pass)
	if pass == 5 {
		fmt.Println(" ✔ 环境就绪")
		os.Exit(0)
	}
	fmt.Println(" —— 按失败项排查：compose/dev/.env 凭据 → docker compose ps → make up-core")
	os.Exit(1)
}
