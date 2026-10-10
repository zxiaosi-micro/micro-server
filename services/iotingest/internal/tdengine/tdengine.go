// Package tdengine · TDengine REST 客户端（S6-03，ADR-13：超级表按 product_key + SN 子表 + 同 ts 覆盖幂等）。
// REST 6041 必须 Basic 认证（E7：401 易误判空结果）。
package tdengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// Point 单条遥测（热点五指标 + raw，ADR-13）。
type Point struct {
	TenantId    int64
	ProductKey  string
	Sn          string
	Ts          time.Time
	Soc         float64
	Voltage     float64
	Current     float64
	Temperature float64
	Power       float64
	Raw         string
}

// Client TDengine REST 客户端。
type Client struct {
	restUrl string
	user    string
	pass    string
	db      string
	http    *http.Client
	stables sync.Map // product_key → 建表已确认（进程内缓存，幂等建超级表只做一次）
}

// New 构造。
func New(restUrl, user, pass, db string) *Client {
	return &Client{
		restUrl: strings.TrimRight(restUrl, "/"),
		user:    user,
		pass:    pass,
		db:      db,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

type taosResp struct {
	Code int    `json:"code"`
	Desc string `json:"desc"`
}

// exec 执行 SQL（Basic 认证，E7）。
func (c *Client) exec(ctx context.Context, sql string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.restUrl+"/rest/sql", strings.NewReader(sql))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.pass)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var out taosResp
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode == 401 {
		return fmt.Errorf("tdengine 401（检查 Basic 认证口令，E7）")
	}
	if out.Code != 0 {
		return fmt.Errorf("tdengine err code=%d desc=%s", out.Code, out.Desc)
	}
	return nil
}

// EnsureDatabase 建库（幂等；启动期调用）。
func (c *Client) EnsureDatabase(ctx context.Context) error {
	if err := c.exec(ctx, "CREATE DATABASE IF NOT EXISTS "+c.db+" PRECISION 'ms' KEEP 90 DURATION 10"); err != nil {
		return err
	}
	// 降采样连续查询（1min，保留 3 年，02 §14.1）不在此建——由 ops 脚本统一管理
	return nil
}

// stableName 超级表名（product_key 转小写合法标识）。
func stableName(pk string) string {
	return "iot_" + sanitize(pk)
}

func tableName(pk, sn string) string {
	return "iot_" + sanitize(pk) + "_" + sanitize(sn)
}

// qualified 表名限定（REST 无连接态默认库，全部显式带 <db>. 前缀——9750 Database not specified 教训）。
func (c *Client) qualifiedStable(pk string) string { return c.db + "." + stableName(pk) }
func (c *Client) qualifiedTable(pk, sn string) string {
	return c.db + "." + tableName(pk, sn)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// ensureStable 幂等建超级表（同 pk 只建一次；热点列 + raw，ADR-13）。
func (c *Client) ensureStable(ctx context.Context, pk string) error {
	if _, ok := c.stables.Load(pk); ok {
		return nil
	}
	sql := fmt.Sprintf(
		"CREATE STABLE IF NOT EXISTS %s (ts TIMESTAMP, soc FLOAT, voltage FLOAT, current FLOAT, temperature FLOAT, power FLOAT, raw VARCHAR(1024)) TAGS (sn VARCHAR(64) , tenant_id BIGINT)",
		c.qualifiedStable(pk))
	if err := c.exec(ctx, sql); err != nil {
		return err
	}
	c.stables.Store(pk, true)
	return nil
}

// WriteBatch 批量写入（同 ts 覆盖 = 遥测重传天然幂等，FR-IOT-003）。
func (c *Client) WriteBatch(ctx context.Context, pts []Point) error {
	if len(pts) == 0 {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("INSERT INTO ")
	for _, p := range pts {
		if err := c.ensureStable(ctx, p.ProductKey); err != nil {
			return err
		}
		// 每块都写全 USING 形式：TDengine 3.3 带库名前缀时，同子表续写块省略 USING 会报
		// 9731 Table does not exist（实测）；重复 USING 幂等无害。
		fmt.Fprintf(&b, "%s USING %s TAGS ('%s', %d) ",
			c.qualifiedTable(p.ProductKey, p.Sn), c.qualifiedStable(p.ProductKey),
			strings.ReplaceAll(p.Sn, "'", "''"), p.TenantId)
		fmt.Fprintf(&b, "VALUES ('%s', %g, %g, %g, %g, %g, '%s') ",
			p.Ts.Format("2006-01-02 15:04:05.000"),
			p.Soc, p.Voltage, p.Current, p.Temperature, p.Power,
			strings.ReplaceAll(truncate(p.Raw, 1000), "'", "''"))
	}
	if err := c.exec(ctx, b.String()); err != nil {
		logx.WithContext(ctx).Errorf("tdengine SQL 失败（前 500 字符）: %.500s", b.String())
		return err
	}
	logx.WithContext(ctx).Debugf("tdengine batch written n=%d", len(pts))
	return nil
}

// Ping 连通检查（envcheck/iotsmoke 用）。
func (c *Client) Ping(ctx context.Context) error {
	return c.exec(ctx, "show databases")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
