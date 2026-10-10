package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 六大日聚合表 upsert（S8-01）：INSERT..ON DUPLICATE KEY UPDATE 增量累加。
// 幂等前提：调用方已过台账 uk_biz 幂等（重复事件不会走到这里），增量累加即精确。
// 金额以 string 传参（DECIMAL 精确累加，不落 float）。

type DailyModel interface {
	// ---- 销售日报 ----
	ApplyOrderCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error
	ApplyOrderPaidTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, channel, amount string) error
	ApplyOrderCancelledTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error
	ApplyOrderPayTimeoutTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error
	ApplyReturnApprovedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error
	ApplyShipmentSignedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, snsCount int64) error
	// ---- 库存日报 ----
	ApplyStockInTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID, qty int64) error
	ApplyStockOutTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID, qty int64) error
	ApplyStockLowTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID int64) error
	// ---- 质保日报 ----
	ApplyWarrantyStartedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, level string) error
	ApplyDeviceActivatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error
	// ---- SLA / 场站日报 ----
	ApplyAlertCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, stationID int64, level string) error
	// ---- 财务日报 ----
	ApplyPaymentRefundedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, channel, amount string) error
	ApplyStationCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, stationID int64) error

	// ---- 查询（读模型服务面）----
	ScanSales(ctx context.Context, tid int64, start, end time.Time) ([]*SalesRow, error)
	ScanInventory(ctx context.Context, tid int64, start, end time.Time, warehouseID int64) ([]*InventoryRow, error)
	ScanWarranty(ctx context.Context, tid int64, start, end time.Time) ([]*WarrantyRow, error)
	ScanSla(ctx context.Context, tid int64, start, end time.Time, stationID int64) ([]*SlaRow, error)
	ScanFinance(ctx context.Context, tid int64, start, end time.Time, channel string) ([]*FinanceRow, error)
	ScanStation(ctx context.Context, tid int64, start, end time.Time, stationID int64) ([]*StationRow, error)

	// SumSalesMetric 对账 model 口径：指定列在业务日的合计。
	SumSalesMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error)
	SumFinanceRows(ctx context.Context, tid int64, date time.Time) (payments int64, paidAmount string, refunds int64, refundAmount string, err error)
	SumInventoryMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error)
	CountWarrantyMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error)
	SumSlaMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error)
	CountStationMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error)

	// TruncateAll 读模型全量重建（RebuildReadModel：台账 + 六表 + watermark 一次清空）。
	TruncateAll(ctx context.Context) error
}

type dailyModel struct{ conn sqlx.SqlConn }

func NewDailyModel(conn sqlx.SqlConn) DailyModel { return &dailyModel{conn: conn} }

// ---------- 行结构 ----------

type SalesRow struct {
	StatDate         time.Time `db:"stat_date"`
	OrdersCreated    int64     `db:"orders_created"`
	OrdersPaid       int64     `db:"orders_paid"`
	OrdersCancelled  int64     `db:"orders_cancelled"`
	OrdersPayTimeout int64     `db:"orders_pay_timeout"`
	ReturnsApproved  int64     `db:"returns_approved"`
	ShipmentsSigned  int64     `db:"shipments_signed"`
	DevicesShipped   int64     `db:"devices_shipped"`
	GmvPaid          string    `db:"gmv_paid"`
}

type InventoryRow struct {
	StatDate       time.Time `db:"stat_date"`
	WarehouseId    int64     `db:"warehouse_id"`
	StockInQty     int64     `db:"stock_in_qty"`
	StockOutQty    int64     `db:"stock_out_qty"`
	LowStockEvents int64     `db:"low_stock_events"`
}

type WarrantyRow struct {
	StatDate          time.Time `db:"stat_date"`
	WarrantiesStarted int64     `db:"warranties_started"`
	WarrantiesDevice  int64     `db:"warranties_device"`
	WarrantiesStation int64     `db:"warranties_station"`
	DevicesActivated  int64     `db:"devices_activated"`
}

type SlaRow struct {
	StatDate    time.Time `db:"stat_date"`
	StationId   int64     `db:"station_id"`
	AlertsTotal int64     `db:"alerts_total"`
	AlertsP1    int64     `db:"alerts_p1"`
	AlertsP2    int64     `db:"alerts_p2"`
	AlertsP3    int64     `db:"alerts_p3"`
}

type FinanceRow struct {
	StatDate      time.Time `db:"stat_date"`
	Channel       string    `db:"channel"`
	PaymentsCount int64     `db:"payments_count"`
	PaidAmount    string    `db:"paid_amount"`
	RefundsCount  int64     `db:"refunds_count"`
	RefundAmount  string    `db:"refund_amount"`
}

type StationRow struct {
	StatDate        time.Time `db:"stat_date"`
	StationId       int64     `db:"station_id"`
	StationsCreated int64     `db:"stations_created"`
	AlertsTotal     int64     `db:"alerts_total"`
	AlertsP1        int64     `db:"alerts_p1"`
}

// ---------- upsert 应用 ----------

func (m *dailyModel) ApplyOrderCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, orders_created) VALUES (?,?,1) "+
			"ON DUPLICATE KEY UPDATE orders_created = orders_created + 1",
		tid, date)
	return err
}

func (m *dailyModel) ApplyOrderPaidTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, channel, amount string) error {
	if _, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, orders_paid, gmv_paid) VALUES (?,?,1,?) "+
			"ON DUPLICATE KEY UPDATE orders_paid = orders_paid + 1, gmv_paid = gmv_paid + VALUES(gmv_paid)",
		tid, date, amount); err != nil {
		return err
	}
	// 财务日报同源累加（channel 维度）
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_finance_daily (tenant_id, stat_date, channel, payments_count, paid_amount) VALUES (?,?,?,1,?) "+
			"ON DUPLICATE KEY UPDATE payments_count = payments_count + 1, paid_amount = paid_amount + VALUES(paid_amount)",
		tid, date, channel, amount)
	return err
}

func (m *dailyModel) ApplyOrderCancelledTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, orders_cancelled) VALUES (?,?,1) "+
			"ON DUPLICATE KEY UPDATE orders_cancelled = orders_cancelled + 1",
		tid, date)
	return err
}

func (m *dailyModel) ApplyOrderPayTimeoutTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, orders_pay_timeout) VALUES (?,?,1) "+
			"ON DUPLICATE KEY UPDATE orders_pay_timeout = orders_pay_timeout + 1",
		tid, date)
	return err
}

func (m *dailyModel) ApplyReturnApprovedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, returns_approved) VALUES (?,?,1) "+
			"ON DUPLICATE KEY UPDATE returns_approved = returns_approved + 1",
		tid, date)
	return err
}

func (m *dailyModel) ApplyShipmentSignedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, snsCount int64) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_sales_daily (tenant_id, stat_date, shipments_signed, devices_shipped) VALUES (?,?,1,?) "+
			"ON DUPLICATE KEY UPDATE shipments_signed = shipments_signed + 1, devices_shipped = devices_shipped + VALUES(devices_shipped)",
		tid, date, snsCount)
	return err
}

func (m *dailyModel) ApplyStockInTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID, qty int64) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_inventory_daily (tenant_id, stat_date, warehouse_id, stock_in_qty) VALUES (?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE stock_in_qty = stock_in_qty + VALUES(stock_in_qty)",
		tid, date, warehouseID, qty)
	return err
}

func (m *dailyModel) ApplyStockOutTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID, qty int64) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_inventory_daily (tenant_id, stat_date, warehouse_id, stock_out_qty) VALUES (?,?,?,?) "+
			"ON DUPLICATE KEY UPDATE stock_out_qty = stock_out_qty + VALUES(stock_out_qty)",
		tid, date, warehouseID, qty)
	return err
}

func (m *dailyModel) ApplyStockLowTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, warehouseID int64) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_inventory_daily (tenant_id, stat_date, warehouse_id, low_stock_events) VALUES (?,?,?,1) "+
			"ON DUPLICATE KEY UPDATE low_stock_events = low_stock_events + 1",
		tid, date, warehouseID)
	return err
}

func (m *dailyModel) ApplyWarrantyStartedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, level string) error {
	levelCol := "warranties_device"
	if level == "STATION" {
		levelCol = "warranties_station"
	}
	_, err := s.ExecCtx(ctx, fmt.Sprintf(
		"INSERT INTO report_warranty_daily (tenant_id, stat_date, warranties_started, %s) VALUES (?,?,1,1) "+
			"ON DUPLICATE KEY UPDATE warranties_started = warranties_started + 1, %s = %s + 1", levelCol, levelCol, levelCol),
		tid, date)
	return err
}

func (m *dailyModel) ApplyDeviceActivatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_warranty_daily (tenant_id, stat_date, devices_activated) VALUES (?,?,1) "+
			"ON DUPLICATE KEY UPDATE devices_activated = devices_activated + 1",
		tid, date)
	return err
}

func (m *dailyModel) ApplyAlertCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, stationID int64, level string) error {
	p1, p2, p3 := 0, 0, 0
	switch level {
	case "P1":
		p1 = 1
	case "P2":
		p2 = 1
	default:
		p3 = 1
	}
	if _, err := s.ExecCtx(ctx,
		"INSERT INTO report_sla_daily (tenant_id, stat_date, station_id, alerts_total, alerts_p1, alerts_p2, alerts_p3) VALUES (?,?,?,1,?,?,?) "+
			"ON DUPLICATE KEY UPDATE alerts_total = alerts_total + 1, alerts_p1 = alerts_p1 + VALUES(alerts_p1), alerts_p2 = alerts_p2 + VALUES(alerts_p2), alerts_p3 = alerts_p3 + VALUES(alerts_p3)",
		tid, date, stationID, p1, p2, p3); err != nil {
		return err
	}
	// 场站日报同源累加（station_id=0 平台级行不进场站表）
	if stationID > 0 {
		_, err := s.ExecCtx(ctx,
			"INSERT INTO report_station_daily (tenant_id, stat_date, station_id, alerts_total, alerts_p1) VALUES (?,?,?,1,?) "+
				"ON DUPLICATE KEY UPDATE alerts_total = alerts_total + 1, alerts_p1 = alerts_p1 + VALUES(alerts_p1)",
			tid, date, stationID, p1)
		return err
	}
	return nil
}

func (m *dailyModel) ApplyPaymentRefundedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, channel, amount string) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_finance_daily (tenant_id, stat_date, channel, refunds_count, refund_amount) VALUES (?,?,?,1,?) "+
			"ON DUPLICATE KEY UPDATE refunds_count = refunds_count + 1, refund_amount = refund_amount + VALUES(refund_amount)",
		tid, date, channel, amount)
	return err
}

func (m *dailyModel) ApplyStationCreatedTx(ctx context.Context, s sqlx.Session, tid int64, date time.Time, stationID int64) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_station_daily (tenant_id, stat_date, station_id, stations_created) VALUES (?,?,?,1) "+
			"ON DUPLICATE KEY UPDATE stations_created = stations_created + 1",
		tid, date, stationID)
	return err
}

// ---------- 查询（服务面） ----------

func (m *dailyModel) ScanSales(ctx context.Context, tid int64, start, end time.Time) ([]*SalesRow, error) {
	var rows []*SalesRow
	err := m.conn.QueryRowsCtx(ctx, &rows,
		"SELECT stat_date, orders_created, orders_paid, orders_cancelled, orders_pay_timeout, returns_approved, shipments_signed, devices_shipped, gmv_paid FROM report_sales_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ? ORDER BY stat_date",
		tid, start, end)
	return rows, err
}

func (m *dailyModel) ScanInventory(ctx context.Context, tid int64, start, end time.Time, warehouseID int64) ([]*InventoryRow, error) {
	query := "SELECT stat_date, warehouse_id, stock_in_qty, stock_out_qty, low_stock_events FROM report_inventory_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ?"
	args := []any{tid, start, end}
	if warehouseID > 0 {
		query += " AND warehouse_id = ?"
		args = append(args, warehouseID)
	}
	query += " ORDER BY stat_date, warehouse_id"
	var rows []*InventoryRow
	err := m.conn.QueryRowsCtx(ctx, &rows, query, args...)
	return rows, err
}

func (m *dailyModel) ScanWarranty(ctx context.Context, tid int64, start, end time.Time) ([]*WarrantyRow, error) {
	var rows []*WarrantyRow
	err := m.conn.QueryRowsCtx(ctx, &rows,
		"SELECT stat_date, warranties_started, warranties_device, warranties_station, devices_activated FROM report_warranty_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ? ORDER BY stat_date",
		tid, start, end)
	return rows, err
}

func (m *dailyModel) ScanSla(ctx context.Context, tid int64, start, end time.Time, stationID int64) ([]*SlaRow, error) {
	query := "SELECT stat_date, station_id, alerts_total, alerts_p1, alerts_p2, alerts_p3 FROM report_sla_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ?"
	args := []any{tid, start, end}
	if stationID > 0 {
		query += " AND station_id = ?"
		args = append(args, stationID)
	}
	query += " ORDER BY stat_date, station_id"
	var rows []*SlaRow
	err := m.conn.QueryRowsCtx(ctx, &rows, query, args...)
	return rows, err
}

func (m *dailyModel) ScanFinance(ctx context.Context, tid int64, start, end time.Time, channel string) ([]*FinanceRow, error) {
	query := "SELECT stat_date, channel, payments_count, paid_amount, refunds_count, refund_amount FROM report_finance_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ?"
	args := []any{tid, start, end}
	if channel != "" {
		query += " AND channel = ?"
		args = append(args, channel)
	}
	query += " ORDER BY stat_date, channel"
	var rows []*FinanceRow
	err := m.conn.QueryRowsCtx(ctx, &rows, query, args...)
	return rows, err
}

func (m *dailyModel) ScanStation(ctx context.Context, tid int64, start, end time.Time, stationID int64) ([]*StationRow, error) {
	query := "SELECT stat_date, station_id, stations_created, alerts_total, alerts_p1 FROM report_station_daily WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ?"
	args := []any{tid, start, end}
	if stationID > 0 {
		query += " AND station_id = ?"
		args = append(args, stationID)
	}
	query += " ORDER BY stat_date, station_id"
	var rows []*StationRow
	err := m.conn.QueryRowsCtx(ctx, &rows, query, args...)
	return rows, err
}

// ---------- 对账 model 口径 ----------

// SumSalesMetric 白名单列名拼接（column 仅来自 recon 内部常量，非用户输入）。
func (m *dailyModel) SumSalesMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		fmt.Sprintf("SELECT IFNULL(SUM(%s),0) FROM report_sales_daily WHERE tenant_id = ? AND stat_date = ?", column),
		tid, date)
	return n, err
}

func (m *dailyModel) SumInventoryMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		fmt.Sprintf("SELECT IFNULL(SUM(%s),0) FROM report_inventory_daily WHERE tenant_id = ? AND stat_date = ?", column),
		tid, date)
	return n, err
}

func (m *dailyModel) CountWarrantyMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		fmt.Sprintf("SELECT IFNULL(SUM(%s),0) FROM report_warranty_daily WHERE tenant_id = ? AND stat_date = ?", column),
		tid, date)
	return n, err
}

func (m *dailyModel) SumSlaMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		fmt.Sprintf("SELECT IFNULL(SUM(%s),0) FROM report_sla_daily WHERE tenant_id = ? AND stat_date = ?", column),
		tid, date)
	return n, err
}

func (m *dailyModel) CountStationMetric(ctx context.Context, tid int64, date time.Time, column string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		fmt.Sprintf("SELECT IFNULL(SUM(%s),0) FROM report_station_daily WHERE tenant_id = ? AND stat_date = ?", column),
		tid, date)
	return n, err
}

func (m *dailyModel) SumFinanceRows(ctx context.Context, tid int64, date time.Time) (int64, string, int64, string, error) {
	var row struct {
		Payments     int64  `db:"payments"`
		PaidAmount   string `db:"paid_amount"`
		Refunds      int64  `db:"refunds"`
		RefundAmount string `db:"refund_amount"`
	}
	err := m.conn.QueryRowCtx(ctx, &row,
		"SELECT IFNULL(SUM(payments_count),0) AS payments, CAST(IFNULL(SUM(paid_amount),0) AS CHAR) AS paid_amount, IFNULL(SUM(refunds_count),0) AS refunds, CAST(IFNULL(SUM(refund_amount),0) AS CHAR) AS refund_amount FROM report_finance_daily WHERE tenant_id = ? AND stat_date = ?",
		tid, date)
	return row.Payments, row.PaidAmount, row.Refunds, row.RefundAmount, err
}

// ---------- 重建 ----------

func (m *dailyModel) TruncateAll(ctx context.Context) error {
	for _, table := range []string{
		"report_event_entry",
		"report_sales_daily", "report_inventory_daily", "report_warranty_daily",
		"report_sla_daily", "report_finance_daily", "report_station_daily",
		"report_watermark",
	} {
		if _, err := m.conn.ExecCtx(ctx, "TRUNCATE TABLE "+table); err != nil {
			return err
		}
	}
	return nil
}
