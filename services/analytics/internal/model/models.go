// Package model · analytics 模型聚合根（一库一连接；对账跨库走 logic 直查业务 DSN，不入模型层）。
package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

// Models analytics_db 全部模型（单连接共享）。
type Models struct {
	Dict      MetricDictModel // 指标字典
	Entry     EventEntryModel // 事件台账（幂等正本）
	Daily     DailyModel      // 六大日聚合读模型
	Watermark WatermarkModel  // 消费水位
	Meta      MetaModel       // 消费组版本元数据
	Export    ExportTaskModel // 异步导出任务
	Recon     ReconModel      // 日级对账差异
	Dw        DwModel         // 数仓星型 ETL
}

// NewModels 按单连接组装全部模型。
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
