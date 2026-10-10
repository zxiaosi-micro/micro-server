// export_view.go · 导出任务行 → pb 视图（logic 层共用）。
package logic

import (
	"strconv"

	"micro-server/services/analytics/internal/model"
	"micro-server/services/analytics/pb"
)

func exportTaskItem(t *model.ExportTask) *pb.ExportTaskItem {
	return &pb.ExportTaskItem{
		ExportId:    strconv.FormatInt(t.ID, 10),
		ReportType:  t.ReportType,
		Format:      t.Format,
		DateStart:   t.DateStart.Format("2006-01-02"),
		DateEnd:     t.DateEnd.Format("2006-01-02"),
		Status:      t.Status,
		FileId:      strconv.FormatInt(t.FileId, 10),
		FileName:    t.FileName,
		RowCount:    t.RowCount,
		CostMs:      t.CostMs,
		Error:       t.Error,
		RequestedBy: strconv.FormatInt(t.RequestedBy, 10),
		CreatedAt:   t.CreatedAt.UnixMilli(),
	}
}
