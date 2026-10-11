// Package model · analytics 模型聚合根（一库一连接；对账跨库走 logic 直查业务 DSN，不入模型层）。
package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

// Models analytics_db 全部模型（单连接共享）。
type Models struct {
	Dict      MetricDictModel
	Entry     EventEntryModel
	Daily     DailyModel
	Watermark WatermarkModel
	Meta      MetaModel
	Export    ExportTaskModel
	Recon     ReconModel
	Dw        DwModel
}

func NewModels(conn sqlx.SqlConn) *Models {
	return &Models{
		Dict:      NewMetricDictModel(conn),
		Entry:     NewEventEntryModel(conn),
		Daily:     NewDailyModel(conn),
		Watermark: NewWatermarkModel(conn),
		Meta:      NewMetaModel(conn),
		Export:    NewExportTaskModel(conn),
		Recon:     NewReconModel(conn),
		Dw:        NewDwModel(conn),
	}
}
