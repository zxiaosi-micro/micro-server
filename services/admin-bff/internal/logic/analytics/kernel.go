// kernel.go · 分析域 BFF 逻辑（S8-05）：转发 analytics RPC + 总览并行扇出 + 导出下载代理。
//
// 总览用 go-zero core/mr.Finish 并行扇出六大报表（02 §9.8「多源聚合 mr.Finish」；
// 任一失败快速失败）。下载走 file RPC GetFileData 取流，base64 信封回传
// （BFF 会话鉴权天然受控，前端解码为 Blob 触发下载）。

package analytics

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	anpb "micro-server/services/analytics/pb"
	filepb "micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

// errAnalyticsFileMissing 导出产物缺失（analytics 段 150000，已由 analytics 服务登记）。
var errAnalyticsFileMissing = errcode.New(150000, 1004, "导出产物缺失")

// reportTypes 六大报表（S8-02 字典口径；顺序即总览返回顺序）。
var reportTypes = []struct {
	Type string
	Name string
}{
	{"sales", "销售日报"},
	{"inventory", "库存日报"},
	{"warranty", "质保日报"},
	{"sla", "SLA 日报"},
	{"finance", "财务日报"},
	{"station", "场站日报"},
}

func i64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// GetReport 报表查询（读模型服务面直通）。
func (l *GetReportLogic) GetReport(req *types.ReportQueryReq) (*types.ReportResp, error) {
	out, err := l.svcCtx.Analytics.GetReport(l.ctx, &anpb.GetReportReq{
		ReportType:  req.Type,
		DateStart:   req.DateStart,
		DateEnd:     req.DateEnd,
		StationId:   i64(req.StationId),
		WarehouseId: i64(req.WarehouseId),
		Channel:     req.Channel,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.ReportResp{Total: out.Total}
	for _, r := range out.Rows {
		row := types.ReportRow{
			StatDate: r.StatDate,
			Metrics:  r.Metrics,
		}
		if r.StationId > 0 {
			row.StationId = strconv.FormatInt(r.StationId, 10)
		}
		if r.WarehouseId > 0 {
			row.WarehouseId = strconv.FormatInt(r.WarehouseId, 10)
		}
		row.Channel = r.Channel
		resp.Rows = append(resp.Rows, row)
	}
	return resp, nil
}

// GetReportOverview 报表总览：六大报表当日合计并行扇出（mr.Finish，任一失败快速失败）。
func (l *GetReportOverviewLogic) GetReportOverview(req *types.ReportOverviewReq) (*types.ReportOverviewResp, error) {
	date := req.Date
	if date == "" {
		date = today()
	}
	items := make([]types.ReportOverviewItem, len(reportTypes))
	funcs := make([]func() error, 0, len(reportTypes))
	for i, rt := range reportTypes {
		funcs = append(funcs, func() error {
			out, err := l.svcCtx.Analytics.GetReport(l.ctx, &anpb.GetReportReq{
				ReportType: rt.Type, DateStart: date, DateEnd: date,
			})
			if err != nil {
				return err
			}
			items[i] = types.ReportOverviewItem{
				ReportType: rt.Type,
				Name:       rt.Name,
				Metrics:    sumMetrics(out.Rows),
			}
			return nil
		})
	}
	if err := mr.Finish(funcs...); err != nil {
		return nil, err
	}
	return &types.ReportOverviewResp{Items: items}, nil
}

// sumMetrics 跨行合计（整数按 int64 累加；金额按 float 累加保留两位——仅总览展示口径，明细走报表查询）。
func sumMetrics(rows []*anpb.ReportRow) map[string]string {
	sumI := map[string]int64{}
	sumF := map[string]float64{}
	isF := map[string]bool{}
	for _, r := range rows {
		for k, vs := range r.Metrics {
			if n, err := strconv.ParseInt(vs, 10, 64); err == nil && !isF[k] {
				sumI[k] += n
				continue
			}
			if f, err := strconv.ParseFloat(vs, 64); err == nil {
				isF[k] = true
				delete(sumI, k)
				sumF[k] += f
			}
		}
	}
	out := map[string]string{}
	for k, v := range sumI {
		out[k] = strconv.FormatInt(v, 10)
	}
	for k, v := range sumF {
		out[k] = strconv.FormatFloat(v, 'f', 2, 64)
	}
	return out
}

// ListMetricDict 指标字典（口径正本透传）。
func (l *ListMetricDictLogic) ListMetricDict(req *types.MetricDictReq) (*types.MetricDictResp, error) {
	out, err := l.svcCtx.Analytics.ListMetricDict(l.ctx, &anpb.ListMetricDictReq{ReportType: req.ReportType})
	if err != nil {
		return nil, err
	}
	resp := &types.MetricDictResp{Total: out.Total}
	for _, d := range out.List {
		resp.List = append(resp.List, types.MetricDictItem{
			MetricCode: d.MetricCode, Name: d.Name, ReportType: d.ReportType,
			Unit: d.Unit, Caliber: d.Caliber, SourceEvent: d.SourceEvent,
			Recon: d.Recon, Sort: d.Sort,
		})
	}
	return resp, nil
}

// RunRecon 手动触发对账。
func (l *RunReconLogic) RunRecon(req *types.ReconReq) (*types.ReconResp, error) {
	out, err := l.svcCtx.Analytics.RunRecon(l.ctx, &anpb.RunReconReq{ReconDate: req.ReconDate})
	if err != nil {
		return nil, err
	}
	return &types.ReconResp{DiffTotal: out.DiffTotal}, nil
}

// RebuildReadModel 读模型全量重建（版本号 +1；重启生效，运维操作）。
func (l *RebuildReadModelLogic) RebuildReadModel() (*types.RebuildResp, error) {
	out, err := l.svcCtx.Analytics.RebuildReadModel(l.ctx, &anpb.RebuildReadModelReq{})
	if err != nil {
		return nil, err
	}
	return &types.RebuildResp{RebuildVersion: out.RebuildVersion}, nil
}

// CreateExport 发起导出（analytics 落任务 + export_requested 事件管道）。
func (l *CreateExportLogic) CreateExport(req *types.ExportCreateReq) (*types.ExportCreateResp, error) {
	out, err := l.svcCtx.Analytics.RequestExport(l.ctx, &anpb.RequestExportReq{
		ReportType: req.ReportType, Format: req.Format,
		DateStart: req.DateStart, DateEnd: req.DateEnd,
	})
	if err != nil {
		return nil, err
	}
	return &types.ExportCreateResp{ExportId: out.ExportId}, nil
}

// ListExports 导出任务列表（下载中心）。
func (l *ListExportsLogic) ListExports(req *types.ExportListReq) (*types.ExportListResp, error) {
	out, err := l.svcCtx.Analytics.ListExportTask(l.ctx, &anpb.ListExportTaskReq{
		Status: req.Status, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.ExportListResp{Total: out.Total}
	for _, t := range out.List {
		resp.List = append(resp.List, exportItem(t))
	}
	return resp, nil
}

// DownloadExport 导出产物下载（file RPC 取流 + base64 信封）。
func (l *DownloadExportLogic) DownloadExport(req *types.ExportDownloadReq) (*types.ExportDownloadResp, error) {
	t, err := l.svcCtx.Analytics.GetExportTask(l.ctx, &anpb.GetExportTaskReq{ExportId: req.Id})
	if err != nil {
		return nil, err
	}
	if t.Task == nil || t.Task.Status != "DONE" {
		return nil, errcode.New(150000, 1006, "导出任务尚未完成")
	}
	fid := i64(t.Task.FileId)
	if fid == 0 {
		return nil, errAnalyticsFileMissing
	}
	tok, err := l.svcCtx.File.GetDownloadToken(l.ctx, &filepb.GetDownloadTokenReq{FileId: fid})
	if err != nil {
		return nil, err
	}
	data, err := l.svcCtx.File.GetFileData(l.ctx, &filepb.GetFileDataReq{Token: tok.Token})
	if err != nil {
		return nil, err
	}
	logx.Infof("analytics-bff: 导出下载 task=%s file=%d bytes=%d", req.Id, fid, len(data.Data))
	return &types.ExportDownloadResp{
		FileId:      t.Task.FileId,
		FileName:    t.Task.FileName,
		ContentType: data.ContentType,
		Data:        base64.StdEncoding.EncodeToString(data.Data),
	}, nil
}

func exportItem(t *anpb.ExportTaskItem) types.ExportItem {
	return types.ExportItem{
		ExportId:   t.ExportId,
		ReportType: t.ReportType,
		Format:     t.Format,
		DateStart:  t.DateStart,
		DateEnd:    t.DateEnd,
		Status:     t.Status,
		FileId:     t.FileId,
		FileName:   t.FileName,
		RowCount:   t.RowCount,
		CostMs:     t.CostMs,
		Error:      t.Error,
		CreatedAt:  t.CreatedAt,
	}
}

func today() string { return time.Now().Format("2006-01-02") }

// ---- goctl 生成等价的 Logic 结构（kernel 模式：实现集中本文件，桩不生成） ----

type GetReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportLogic {
	return &GetReportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetReportOverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetReportOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportOverviewLogic {
	return &GetReportOverviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListMetricDictLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMetricDictLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMetricDictLogic {
	return &ListMetricDictLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type RunReconLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRunReconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunReconLogic {
	return &RunReconLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type RebuildReadModelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRebuildReadModelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RebuildReadModelLogic {
	return &RebuildReadModelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateExportLogic {
	return &CreateExportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListExportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListExportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListExportsLogic {
	return &ListExportsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type DownloadExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDownloadExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DownloadExportLogic {
	return &DownloadExportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
