// export.go · 异步导出内核（S8-03）：export_requested 事件 → export_task → CSV/XLSX/PDF → file 服务 → 下载中心。
//
// 并发限流：渲染走 svc.ExportRunner（threading.TaskRunner worker 池，workers=confcenter 可热调），
// 与 RPC goroutine 隔离——10 万行大导出对在线报表请求零影响（FR-ANA-003，p4load 验证）。
// PDF 走 Go 原生库（ADR-18，go-pdf/fpdf + UTF8 中文字体，简单表格版式，行数封顶）。

package logic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"micro-server/services/analytics/internal/confcenter"
	"micro-server/services/analytics/internal/model"
	"micro-server/services/analytics/internal/svc"
	filepb "micro-server/services/file/pb"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// RequestExportCore 导出申请内核（RPC 入口）：任务落库 + export_requested 事件同事务。
func RequestExportCore(ctx context.Context, sc *svc.ServiceContext, tid int64, reportType, format, dateStart, dateEnd string, requestedBy int64) (int64, error) {
	if !validReportType(reportType) {
		return 0, errReportTypeBad
	}
	if format != "CSV" && format != "XLSX" && format != "PDF" {
		return 0, errExportFormatBad
	}
	start, err := parseDay(dateStart)
	if err != nil {
		return 0, errDateFormatBad
	}
	end, err := parseDay(dateEnd)
	if err != nil {
		return 0, errDateFormatBad
	}
	if end.Before(start) {
		return 0, errDateFormatBad.WithMsg("date_end 早于 date_start")
	}

	taskID := sc.Snowflake.MustNextID()
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Export.InsertInTx(ctx, session, &model.ExportTask{
			ID: taskID, TenantId: tid, ReportType: reportType, Format: format,
			DateStart: start, DateEnd: end, Status: "PENDING", RequestedBy: requestedBy,
		}); err != nil {
			return err
		}
		// Outbox 同事务（Relay → Kafka export_requested → 本服务消费渲染）
		return eventbus.Emit(ctx, session, eventbus.EmitInput{
			Topic:    eventbus.TopicExportRequested,
			Type:     eventbus.TypeExportRequested,
			Key:      strconv.FormatInt(taskID, 10),
			TenantID: tid,
			Payload: map[string]any{
				"task_id": taskID, "report_type": reportType, "format": format,
			},
		})
	})
	if err != nil {
		return 0, err
	}
	logx.Infof("analytics: 导出申请 task=%d tenant=%d type=%s format=%s range=%s~%s",
		taskID, tid, reportType, format, dateStart, dateEnd)
	return taskID, nil
}

// scheduleExport 消费侧调度：CAS 抢占 → TaskRunner 排队渲染（handler 不阻塞消费循环）。
func scheduleExport(ctx context.Context, sc *svc.ServiceContext, taskID int64) {
	task, err := loadTaskAnyTenant(ctx, sc, taskID)
	if err != nil {
		logx.Errorf("analytics: 导出任务加载失败 task=%d: %v", taskID, err)
		return
	}
	ok, err := sc.Models.Export.CasStatus(ctx, taskID, "PENDING", "RUNNING")
	if err != nil {
		logx.Errorf("analytics: 导出任务抢占失败 task=%d: %v", taskID, err)
		return
	}
	if !ok {
		logx.Infof("analytics: 导出任务已被抢占/终态 task=%d status=%s", taskID, task.Status)
		return
	}
	sc.ExportRunner.Schedule(func() { renderAndExport(sc, task) })
}

// loadTaskAnyTenant 消费侧任务加载（事件自带租户归属；渲染按任务行执行，非用户请求面）。
func loadTaskAnyTenant(ctx context.Context, sc *svc.ServiceContext, taskID int64) (*model.ExportTask, error) {
	var t model.ExportTask
	err := sc.Conn.QueryRowCtx(ctx, &t,
		"SELECT id, tenant_id, report_type, format, date_start, date_end, status, file_id, file_name, row_count, cost_ms, error, requested_by, created_at, updated_at FROM export_task WHERE id = ?",
		taskID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// renderAndExport 渲染 + 归档 + 状态回写（TaskRunner worker 内执行）。
// ctx 按任务归属租户重建（消费 handler 的 ctx 在返回后不可靠；file RPC 出站桥依赖租户元数据）。
func renderAndExport(sc *svc.ServiceContext, task *model.ExportTask) {
	ctx := tenantx.WithTenant(context.Background(), task.TenantId)
	start := time.Now()

	table, err := buildExportTable(ctx, sc, task)
	if err != nil {
		logx.Errorf("analytics: 导出取数失败 task=%d: %v", task.ID, err)
		_ = sc.Models.Export.UpdateFailed(ctx, task.ID, truncErr(err.Error()))
		return
	}

	var (
		data        []byte
		contentType string
		rerr        error
	)
	switch task.Format {
	case "CSV":
		data, contentType, rerr = renderCSV(table)
	case "XLSX":
		data, contentType, rerr = renderXLSX(table)
	case "PDF":
		data, contentType, rerr = renderPDF(table, pdfFontOf(sc))
	default:
		rerr = errExportFormatBad
	}
	if rerr != nil {
		logx.Errorf("analytics: 导出渲染失败 task=%d format=%s: %v", task.ID, task.Format, rerr)
		_ = sc.Models.Export.UpdateFailed(ctx, task.ID, truncErr(rerr.Error()))
		return
	}

	// 归档 file 服务（biz_type=export → 普通桶）
	if sc.File == nil {
		_ = sc.Models.Export.UpdateFailed(ctx, task.ID, "file RPC 未配置，产物无法归档")
		return
	}
	put, err := sc.File.PutFile(ctx, &filepb.PutFileReq{
		Name: exportFileName(task), ContentType: contentType,
		BizType: "export", BizId: strconv.FormatInt(task.ID, 10),
		Data: data,
	})
	if err != nil {
		logx.Errorf("analytics: 导出归档失败 task=%d: %v", task.ID, err)
		_ = sc.Models.Export.UpdateFailed(ctx, task.ID, truncErr("归档失败: "+err.Error()))
		return
	}

	if err := sc.Models.Export.UpdateDone(ctx, task.ID, put.FileId, exportFileName(task), int64(len(table.rows)), time.Since(start).Milliseconds()); err != nil {
		logx.Errorf("analytics: 导出完成回写失败 task=%d: %v", task.ID, err)
		return
	}
	logx.Infof("analytics: 导出完成 task=%d rows=%d cost=%dms file=%d",
		task.ID, len(table.rows), time.Since(start).Milliseconds(), put.FileId)
}

// exportTable 渲染中间结构。
type exportTable struct {
	name    string
	headers []string
	rows    [][]string
}

// buildExportTable 按报表类型取数（读模型服务面：只查 analytics 本库）。
func buildExportTable(ctx context.Context, sc *svc.ServiceContext, task *model.ExportTask) (*exportTable, error) {
	tid := task.TenantId
	switch task.ReportType {
	case "sales":
		rows, err := sc.Models.Daily.ScanSales(ctx, tid, task.DateStart, task.DateEnd)
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "销售日报", headers: []string{"业务日", "新增订单数", "支付完成订单数", "取消订单数", "支付超时数", "退货审批通过数", "签收发运单数", "签收设备数", "实付GMV"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), itoa64(r.OrdersCreated), itoa64(r.OrdersPaid), itoa64(r.OrdersCancelled), itoa64(r.OrdersPayTimeout), itoa64(r.ReturnsApproved), itoa64(r.ShipmentsSigned), itoa64(r.DevicesShipped), r.GmvPaid})
		}
		return t, nil
	case "inventory":
		rows, err := sc.Models.Daily.ScanInventory(ctx, tid, task.DateStart, task.DateEnd, 0)
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "库存日报", headers: []string{"业务日", "仓库ID", "入库数量", "出库数量", "低库存触发次数"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), itoa64(r.WarehouseId), itoa64(r.StockInQty), itoa64(r.StockOutQty), itoa64(r.LowStockEvents)})
		}
		return t, nil
	case "warranty":
		rows, err := sc.Models.Daily.ScanWarranty(ctx, tid, task.DateStart, task.DateEnd)
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "质保日报", headers: []string{"业务日", "质保起算数", "设备质保", "场站质保", "设备激活数"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), itoa64(r.WarrantiesStarted), itoa64(r.WarrantiesDevice), itoa64(r.WarrantiesStation), itoa64(r.DevicesActivated)})
		}
		return t, nil
	case "sla":
		rows, err := sc.Models.Daily.ScanSla(ctx, tid, task.DateStart, task.DateEnd, 0)
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "SLA日报", headers: []string{"业务日", "场站ID", "告警数", "P1", "P2", "P3"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), itoa64(r.StationId), itoa64(r.AlertsTotal), itoa64(r.AlertsP1), itoa64(r.AlertsP2), itoa64(r.AlertsP3)})
		}
		return t, nil
	case "finance":
		rows, err := sc.Models.Daily.ScanFinance(ctx, tid, task.DateStart, task.DateEnd, "")
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "财务日报", headers: []string{"业务日", "渠道", "支付笔数", "实付金额", "退款笔数", "退款金额"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), r.Channel, itoa64(r.PaymentsCount), r.PaidAmount, itoa64(r.RefundsCount), r.RefundAmount})
		}
		return t, nil
	case "station":
		rows, err := sc.Models.Daily.ScanStation(ctx, tid, task.DateStart, task.DateEnd, 0)
		if err != nil {
			return nil, err
		}
		t := &exportTable{name: "场站日报", headers: []string{"业务日", "场站ID", "新建场站数", "告警数", "P1告警数"}}
		for _, r := range rows {
			t.rows = append(t.rows, []string{day(r.StatDate), itoa64(r.StationId), itoa64(r.StationsCreated), itoa64(r.AlertsTotal), itoa64(r.AlertsP1)})
		}
		return t, nil
	case "event_ledger":
		return buildLedgerTable(ctx, sc, task)
	default:
		return nil, errReportTypeBad
	}
}

// buildLedgerTable 事件流水导出（分页扫描，10 万行级大数据量走这里，p4load 验证对象）。
func buildLedgerTable(ctx context.Context, sc *svc.ServiceContext, task *model.ExportTask) (*exportTable, error) {
	t := &exportTable{name: "事件流水", headers: []string{"台账ID", "业务日", "事件类型", "业务键", "事件ID", "Payload"}}
	t.rows = make([][]string, 0, 100000)
	lastID := int64(0)
	for {
		rows, err := sc.Models.Entry.ListRange(ctx, task.TenantId, task.DateStart, task.DateEnd, lastID, 5000)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			t.rows = append(t.rows, []string{itoa64(r.ID), day(r.StatDate), r.EventType, r.BizKey, r.EventID, r.Payload})
			lastID = r.ID
		}
	}
	return t, nil
}

// ---------- 渲染器 ----------

func renderCSV(t *exportTable) ([]byte, string, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM（Excel 直开中文不乱码）
	bw := bufio.NewWriterSize(&buf, 1<<20) // 1MB 缓冲：10 万行级避免小 flushed
	w := csv.NewWriter(bw)
	if err := w.Write(t.headers); err != nil {
		return nil, "", err
	}
	for _, row := range t.rows {
		if err := w.Write(row); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	if err := bw.Flush(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "text/csv; charset=utf-8", nil
}

func renderXLSX(t *exportTable) ([]byte, string, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, "", err
	}
	headerRow := make([]any, len(t.headers))
	for i, h := range t.headers {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return nil, "", err
	}
	for i, row := range t.rows {
		cells := make([]any, len(row))
		for j, v := range row {
			cells[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, cells); err != nil {
			return nil, "", err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

func renderPDF(t *exportTable, fontPath string) ([]byte, string, error) {
	if fontPath == "" {
		return nil, "", errPdfFontMissing
	}
	// ADR-18：简单表格版式 Go 原生渲染；行数封顶（复杂版式/超大数据走 CSV/XLSX）
	maxRows := confcenter.Current().ExportPdfRows
	if maxRows <= 0 {
		maxRows = 2000
	}
	rows := t.rows
	truncated := false
	if len(rows) > maxRows {
		rows = rows[:maxRows]
		truncated = true
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	if pdf.Err() {
		return nil, "", fmt.Errorf("pdf 初始化失败: %s", pdf.Error())
	}
	pdf.AddUTF8Font("simhei", "", fontPath)
	pdf.SetFont("simhei", "", 9)
	pdf.SetAutoPageBreak(true, 10)

	colW := 190.0 / float64(len(t.headers))
	drawHeader := func() {
		pdf.SetFont("simhei", "", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(190, 7, t.name, "0", 1, "L", false, 0, "")
		for _, h := range t.headers {
			pdf.CellFormat(colW, 6, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.AddPage()
	drawHeader()
	pdf.SetFont("simhei", "", 8)
	for _, row := range rows {
		if pdf.GetY() > 275 {
			pdf.AddPage()
			drawHeader()
			pdf.SetFont("simhei", "", 8)
		}
		for _, v := range row {
			pdf.CellFormat(colW, 5, truncateCell(v), "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	if truncated {
		pdf.Ln(2)
		pdf.CellFormat(0, 5, fmt.Sprintf("（PDF 行数封顶 %d，完整数据请导出 CSV/XLSX）", maxRows), "", 1, "L", false, 0, "")
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "application/pdf", nil
}

func truncateCell(s string) string {
	r := []rune(s)
	if len(r) > 20 {
		return string(r[:19]) + "…"
	}
	return s
}

// ---------- 工具 ----------

func validReportType(t string) bool {
	switch t {
	case "sales", "inventory", "warranty", "sla", "finance", "station", "event_ledger":
		return true
	}
	return false
}

func exportFileName(task *model.ExportTask) string {
	return fmt.Sprintf("export_%s_%s_%s.%s",
		task.ReportType, task.DateStart.Format("20060102"), task.DateEnd.Format("20060102"),
		strings.ToLower(task.Format))
}

func day(t time.Time) string { return t.Format("2006-01-02") }

func truncErr(s string) string {
	r := []rune(s)
	if len(r) > 250 {
		return string(r[:250])
	}
	return s
}

// pdfFontOf 字体路径：confcenter 覆盖 > 启动配置。
func pdfFontOf(sc *svc.ServiceContext) string {
	if v := confcenter.Current().PdfFont; v != "" {
		return v
	}
	return sc.Config.PdfFont
}
