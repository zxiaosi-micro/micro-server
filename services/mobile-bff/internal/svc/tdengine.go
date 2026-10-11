package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
)

func ctxkitWithUIDSID(ctx context.Context, claims *jwtauth.Claims) context.Context {
	ctx = ctxkit.WithUID(ctx, claims.UID)
	ctx = ctxkit.WithSID(ctx, claims.SID)
	ctx = ctxkit.WithClient(ctx, claims.Client)
	return ctx
}

// TdClient TDengine REST 只读客户端（遥测趋势；基本认证 E7、响应 JSON 对象、
// 表名带库名前缀——与 iotingest 同一套坑位口径，AGENTS.md 已知坑）。
type TdClient struct {
	restUrl string
	user    string
	pass    string
	db      string
	http    *http.Client
}

func NewTdClient(restUrl, user, pass, db string) *TdClient {
	return &TdClient{
		restUrl: strings.TrimRight(restUrl, "/"),
		user:    user,
		pass:    pass,
		db:      db,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

type taosResp struct {
	Code    int      `json:"code"`
	Desc    string   `json:"desc"`
	Data    [][]any  `json:"data"`
	Rows    int      `json:"rows"`
	Columns []any    `json:"column_meta"` // 实际返回 [[name,type,len],...]；宽松解析
}

// Query 执行只读 SQL，返回行集（空结果 nil）。
func (c *TdClient) Query(ctx context.Context, sql string) ([][]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.restUrl+"/rest/sql", strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.pass)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("tdengine 401（检查 Basic 认证口令，E7）")
	}
	var out taosResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("tdengine 响应解析失败: %w", err)
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("tdengine err code=%d desc=%s", out.Code, out.Desc)
	}
	return out.Data, nil
}

// Ping 连通检查（/healthz 深检用）。
func (c *TdClient) Ping(ctx context.Context) error {
	_, err := c.Query(ctx, "show databases")
	return err
}

// qualified 子表全名（库前缀；sanitize 与 iotingest 同款）。
func (c *TdClient) qualified(productKey, sn string) string {
	return c.db + ".iot_" + sanitizeIdent(productKey) + "_" + sanitizeIdent(sn)
}

func sanitizeIdent(s string) string {
	var b bytes.Buffer
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// Trend 查询设备热点五指标降采样趋势（1min 桶 last 值）。
func (c *TdClient) Trend(ctx context.Context, productKey, sn string, hours int64) ([][]any, error) {
	// 表不存在时 TDengine 返回非 0 code——趋势页面对无数据设备应返回空而非报错
	table := c.qualified(productKey, sn)
	sql := fmt.Sprintf(
		"select _wstart, last(soc), last(voltage), last(current), last(temperature), last(power) from %s where ts >= now - %dh interval(1m)",
		table, hours)
	rows, err := c.Query(ctx, sql)
	if err != nil {
		logx.WithContext(ctx).Debugf("tdengine trend 查询失败（无数据设备视为空）: %v", err)
		return nil, nil
	}
	return rows, nil
}
