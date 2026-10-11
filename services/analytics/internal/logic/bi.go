// bi.go · BI 自助白名单查询引擎（S10-02）。
//
// 防注入三重设计：
//   1. 标识符（表/列/聚合/操作符）全部经白名单 map 查表得到，用户输入只作 key 不入 SQL；
//   2. 值一律 `?` 占位符绑定（LIKE 通配符由用户值自带，无拼接）；
//   3. LIMIT 恒下发（≤200）+ 分页 offset 参数化，杜绝无界扫描。
// 查询面走 RawDB（database/sql 动态列扫描；BI 为 admin 管理面低频查询，
// 不计入 go-zero sqlx 熔断统计——业务面读模型查询仍走 Conn）。

package logic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// ---------- 白名单注册表（表/列/维度/聚合/操作符） ----------

type biColumn struct {
	Name    string
	Kind    string // DIM/METRIC/BOTH
	Aggs    []string
	Ops     []string
	Comment string
}

type biTable struct {
	Name    string
	Comment string
	Columns map[string]*biColumn
	Order   []string // 白名单展示序
}

// biOps 全量操作符 → SQL 映射（值一律占位符）。
var biOps = map[string]string{
	"EQ": "=", "NE": "<>", "LT": "<", "LTE": "<=", "GT": ">", "GTE": ">=",
	"LIKE": "LIKE", "IN": "IN",
}

// biWhitelist 白名单正本（验收：白名单外查询被拒）。
var biWhitelist = map[string]*biTable{
	"dw_dim_date": {
		Name: "dw_dim_date", Comment: "日历维（年/季/月/周/星期）",
		Columns: map[string]*biColumn{
			"date_key":     {Name: "date_key", Kind: "DIM", Ops: biOpsRange(), Comment: "日期 yyyy-MM-dd"},
			"year":         {Name: "year", Kind: "DIM", Ops: biOpsRange(), Comment: "年"},
			"quarter":      {Name: "quarter", Kind: "DIM", Ops: biOpsRange(), Comment: "季 1~4"},
			"month":        {Name: "month", Kind: "DIM", Ops: biOpsRange(), Comment: "月 1~12"},
			"day_of_month": {Name: "day_of_month", Kind: "DIM", Ops: biOpsRange(), Comment: "日 1~31"},
			"week_of_year": {Name: "week_of_year", Kind: "DIM", Ops: biOpsRange(), Comment: "年内周序"},
			"weekday":      {Name: "weekday", Kind: "DIM", Ops: biOpsRange(), Comment: "星期 1=周一…7=周日"},
			"is_weekend":   {Name: "is_weekend", Kind: "DIM", Ops: []string{"EQ"}, Comment: "是否周末 0/1"},
		},
		Order: []string{"date_key", "year", "quarter", "month", "day_of_month", "week_of_year", "weekday", "is_weekend"},
	},
	"dw_dim_tenant": {
		Name: "dw_dim_tenant", Comment: "租户维",
		Columns: map[string]*biColumn{
			"tenant_id":     {Name: "tenant_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "租户 ID"},
			"first_seen_at": {Name: "first_seen_at", Kind: "DIM", Ops: biOpsRange(), Comment: "首次出现在事实源的时刻"},
		},
		Order: []string{"tenant_id", "first_seen_at"},
	},
	"dw_dim_party": {
		Name: "dw_dim_party", Comment: "参与方维（party 快照）",
		Columns: map[string]*biColumn{
			"party_id":  {Name: "party_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "参与方 ID"},
			"tenant_id": {Name: "tenant_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "归属租户"},
			"name":      {Name: "name", Kind: "DIM", Ops: []string{"EQ", "LIKE"}, Comment: "名称快照"},
			"types":     {Name: "types", Kind: "DIM", Ops: []string{"EQ", "LIKE"}, Comment: "类型 JSON 快照"},
			"region":    {Name: "region", Kind: "DIM", Ops: []string{"EQ", "LIKE"}, Comment: "区域快照"},
		},
		Order: []string{"party_id", "tenant_id", "name", "types", "region"},
	},
	"dw_fact_order": {
		Name: "dw_fact_order", Comment: "订单事实（下单日口径；状态/金额随源刷新）",
		Columns: map[string]*biColumn{
			"order_no":          {Name: "order_no", Kind: "DIM", Ops: []string{"EQ", "LIKE", "IN"}, Comment: "订单号"},
			"date_key":          {Name: "date_key", Kind: "DIM", Ops: biOpsRange(), Comment: "下单日期"},
			"tenant_id":         {Name: "tenant_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "租户"},
			"buyer_party_id":    {Name: "buyer_party_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "买方参与方"},
			"status":            {Name: "status", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "订单状态"},
			"total_amount":      {Name: "total_amount", Kind: "BOTH", Aggs: biAggsNum(), Ops: biOpsRange(), Comment: "订单总额"},
			"source_created_at": {Name: "source_created_at", Kind: "DIM", Ops: biOpsRange(), Comment: "源创建时刻"},
		},
		Order: []string{"order_no", "date_key", "tenant_id", "buyer_party_id", "status", "total_amount", "source_created_at"},
	},
	"dw_fact_payment": {
		Name: "dw_fact_payment", Comment: "支付事实（实收日口径；渠道/状态快照）",
		Columns: map[string]*biColumn{
			"payment_no":  {Name: "payment_no", Kind: "DIM", Ops: []string{"EQ", "LIKE", "IN"}, Comment: "支付单号"},
			"date_key":    {Name: "date_key", Kind: "DIM", Ops: biOpsRange(), Comment: "业务日期（实收日）"},
			"tenant_id":   {Name: "tenant_id", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "租户"},
			"order_no":    {Name: "order_no", Kind: "DIM", Ops: []string{"EQ", "LIKE", "IN"}, Comment: "关联订单"},
			"channel":     {Name: "channel", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "渠道 WECHAT/ALIPAY/BANK_OFFLINE"},
			"status":      {Name: "status", Kind: "DIM", Ops: []string{"EQ", "IN"}, Comment: "支付状态"},
			"amount":      {Name: "amount", Kind: "BOTH", Aggs: biAggsNum(), Ops: biOpsRange(), Comment: "应收金额"},
			"paid_amount": {Name: "paid_amount", Kind: "BOTH", Aggs: biAggsNum(), Ops: biOpsRange(), Comment: "实收金额"},
			"paid_at":     {Name: "paid_at", Kind: "DIM", Ops: biOpsRange(), Comment: "实收时刻"},
		},
		Order: []string{"payment_no", "date_key", "tenant_id", "order_no", "channel", "status", "amount", "paid_amount", "paid_at"},
	},
}

func biOpsRange() []string { return []string{"EQ", "LT", "LTE", "GT", "GTE"} }
func biAggsNum() []string  { return []string{"SUM", "COUNT", "AVG", "MIN", "MAX"} }

// biLimit 引擎上限（防滥用）。
const (
	biMaxDims    = 8
	biMaxMetrics = 8
	biMaxFilters = 10
	biMaxSize    = 200
	biMaxValue   = 256
)

// ---------- RPC 入口 ----------

// ListBIWhitelist 白名单元数据（前端查询构建器数据源）。
func (l *ListBIWhitelistLogic) ListBIWhitelist(_ *pb.ListBIWhitelistReq) (*pb.ListBIWhitelistResp, error) {
	if _, err := mustTenant(l.ctx); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(biWhitelist))
	for name := range biWhitelist {
		names = append(names, name)
	}
	sort.Strings(names)
	resp := &pb.ListBIWhitelistResp{Tables: []*pb.BIWhitelistTable{}}
	for _, name := range names {
		t := biWhitelist[name]
		tw := &pb.BIWhitelistTable{Table: t.Name, Comment: t.Comment, Columns: []*pb.BIWhitelistColumn{}}
		for _, col := range t.Order {
			c := t.Columns[col]
			tw.Columns = append(tw.Columns, &pb.BIWhitelistColumn{
				Column:  c.Name,
				Kind:    c.Kind,
				Aggs:    strings.Join(c.Aggs, ","),
				Ops:     strings.Join(c.Ops, ","),
				Comment: c.Comment,
			})
		}
		resp.Tables = append(resp.Tables, tw)
	}
	return resp, nil
}

// RunBIQuery 白名单查询执行（校验构建 → 参数化执行 → 全 string 行）。
func (l *RunBIQueryLogic) RunBIQuery(in *pb.RunBIQueryReq) (*pb.RunBIQueryResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	q, args, countQ, countArgs, cols, err := biBuild(in)
	if err != nil {
		return nil, err
	}
	rows, total, err := biExec(l.ctx, l.svcCtx, q, args, countQ, countArgs)
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("bi query tenant=%d table=%s dims=%d metrics=%d rows=%d total=%d",
		tid, in.Table, len(in.Dimensions), len(in.Metrics), len(rows), total)
	return &pb.RunBIQueryResp{Columns: cols, Rows: rows, Total: total}, nil
}

// ---------- 校验与构建 ----------

// biBuild 校验 + SQL 构建（标识符全走白名单，值全占位符）。
// 返回主查询/参数 与 组数查询/参数（非分组查询 countQ 为空串，total 恒 1）。
func biBuild(in *pb.RunBIQueryReq) (query string, args []any, countQ string, countArgs []any, cols []string, err error) {
	t, ok := biWhitelist[in.Table]
	if !ok {
		return "", nil, "", nil, nil, errBITableBad.WithMsg("表不在白名单: " + in.Table)
	}
	if len(in.Dimensions) == 0 && len(in.Metrics) == 0 {
		return "", nil, "", nil, nil, errBIQueryEmpty
	}
	if len(in.Dimensions) > biMaxDims || len(in.Metrics) > biMaxMetrics || len(in.Filters) > biMaxFilters {
		return "", nil, "", nil, nil, errBITooMany
	}
	if in.Size < 1 {
		in.Size = 20
	}
	if in.Size > biMaxSize {
		in.Size = biMaxSize
	}
	if in.Page < 1 {
		in.Page = 1
	}

	// 维度（去重查表）
	dims := make([]*biColumn, 0, len(in.Dimensions))
	seen := map[string]bool{}
	for _, d := range in.Dimensions {
		c := t.Columns[d]
		if c == nil || (c.Kind != "DIM" && c.Kind != "BOTH") {
			return "", nil, "", nil, nil, errBIColumnBad.WithMsg("维度列不在白名单: " + d)
		}
		if seen[d] {
			return "", nil, "", nil, nil, errBIColumnBad.WithMsg("维度列重复: " + d)
		}
		seen[d] = true
		dims = append(dims, c)
	}

	// 指标（AGG:col / COUNT:*）
	type biMetric struct{ agg, col, alias string }
	metrics := make([]biMetric, 0, len(in.Metrics))
	for _, m := range in.Metrics {
		agg, col, found := strings.Cut(m, ":")
		if !found {
			return "", nil, "", nil, nil, errBIAggBad.WithMsg("指标须为 AGG:col 形式: " + m)
		}
		if agg == "COUNT" && col == "*" {
			metrics = append(metrics, biMetric{agg: "COUNT", col: "*", alias: "COUNT:*"})
			continue
		}
		c := t.Columns[col]
		if c == nil || (c.Kind != "METRIC" && c.Kind != "BOTH") {
			return "", nil, "", nil, nil, errBIColumnBad.WithMsg("指标列不在白名单: " + col)
		}
		if !contains(c.Aggs, agg) {
			return "", nil, "", nil, nil, errBIAggBad.WithMsg("聚合不在该列白名单: " + m)
		}
		metrics = append(metrics, biMetric{agg: agg, col: c.Name, alias: agg + ":" + c.Name})
	}

	// 过滤（列/操作符查表，值占位符）
	where := "1=1"
	var wargs []any
	for _, f := range in.Filters {
		c := t.Columns[f.Column]
		if c == nil {
			return "", nil, "", nil, nil, errBIColumnBad.WithMsg("过滤列不在白名单: " + f.Column)
		}
		opSql, ok := biOps[f.Op]
		if !ok || !contains(c.Ops, f.Op) {
			return "", nil, "", nil, nil, errBIOpBad.WithMsg("操作符不在该列白名单: " + f.Column + " " + f.Op)
		}
		if len(f.Values) == 0 {
			return "", nil, "", nil, nil, errBIOpBad.WithMsg("过滤值缺失: " + f.Column)
		}
		for _, v := range f.Values {
			if len(v) > biMaxValue {
				return "", nil, "", nil, nil, errBIOpBad.WithMsg("过滤值超长: " + f.Column)
			}
		}
		if f.Op == "IN" {
			ph := make([]string, len(f.Values))
			for i, v := range f.Values {
				ph[i] = "?"
				wargs = append(wargs, v)
			}
			where += " and `" + c.Name + "` in (" + strings.Join(ph, ",") + ")"
			continue
		}
		if len(f.Values) != 1 {
			return "", nil, "", nil, nil, errBIOpBad.WithMsg("单值操作符只接受一个值: " + f.Column)
		}
		where += " and `" + c.Name + "` " + opSql + " ?"
		wargs = append(wargs, f.Values[0])
	}

	// SELECT 列与别名
	var sel []string
	for _, c := range dims {
		sel = append(sel, "`"+c.Name+"`")
		cols = append(cols, c.Name)
	}
	for _, m := range metrics {
		if m.col == "*" {
			sel = append(sel, "count(*) as `"+m.alias+"`")
		} else {
			sel = append(sel, m.agg+"(`"+m.col+"`) as `"+m.alias+"`")
		}
		cols = append(cols, m.alias)
	}

	groupBy := ""
	if len(dims) > 0 {
		var gs []string
		for _, c := range dims {
			gs = append(gs, "`"+c.Name+"`")
		}
		groupBy = " group by " + strings.Join(gs, ",")
	}
	from := "from `" + t.Name + "` where " + where + groupBy

	// 组数查询（分组）；非分组全局聚合恒 1 行。
	if len(dims) > 0 {
		countQ = "select count(*) from (select 1 " + from + ") t"
		countArgs = wargs
	}

	// 排序（维度列或指标别名）
	order := ""
	if in.OrderBy != "" {
		valid := seen[in.OrderBy]
		for _, m := range metrics {
			if m.alias == in.OrderBy {
				valid = true
			}
		}
		if !valid {
			return "", nil, "", nil, nil, errBIOrderBad
		}
		dir := "asc"
		if in.OrderDesc {
			dir = "desc"
		}
		order = " order by `" + in.OrderBy + "` " + dir
	}
	query = "select " + strings.Join(sel, ",") + " " + from + order + " limit ? offset ?"
	args = append(append([]any{}, wargs...), in.Size, (in.Page-1)*in.Size)
	return query, args, countQ, countArgs, cols, nil
}

// biExec 主查询 + 组数查询（RawDB 动态列扫描；值全 string 化）。
func biExec(ctx context.Context, sc *svc.ServiceContext, q string, args []any, countQ string, countArgs []any) ([]*pb.BIQueryRow, int64, error) {
	db, err := sc.Conn.RawDB()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if countQ == "" {
		total = 1
	} else if err := db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }() // errcheck：Close 错误无业务语义（读尽后关闭）
	colNames, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}
	out := make([]*pb.BIQueryRow, 0, 16)
	for rows.Next() {
		vals := make([]any, len(colNames))
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, 0, err
		}
		row := &pb.BIQueryRow{Values: make([]string, len(vals))}
		for i, v := range vals {
			row.Values[i] = biFormat(v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// biFormat driver.Value → string（DECIMAL []byte/时间/数值统一防精度丢失）。
func biFormat(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02 15:04:05")
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ---------- logic 结构体（goctl 骨架等价物，方法体在本文件） ----------

type ListBIWhitelistLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBIWhitelistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBIWhitelistLogic {
	return &ListBIWhitelistLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type RunBIQueryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRunBIQueryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunBIQueryLogic {
	return &RunBIQueryLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}
