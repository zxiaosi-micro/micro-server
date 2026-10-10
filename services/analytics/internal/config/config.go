package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

// Config analytics 服务配置（S8）。
type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource string
	}
	// Kafka 读模型消费组（版本号后缀运行时追加：analytics-read-v{N}）。
	Kafka struct {
		Brokers     []string
		Group       string `json:",default=analytics-read"`
		ExportGroup string `json:",default=analytics-export"`
	}
	// FileRpc 导出产物归档（PutFile；未配置 = 导出落地失败记 error）。
	FileRpc zrpc.RpcClientConf `json:",optional"`
	// 导出并发（threading.TaskRunner worker 数，FR-ANA-003 对在线请求零影响）。
	ExportWorkers int `json:",default=2"`
	// PDF 单文件行数上限（简单表格版式，ADR-18；超出走 CSV/XLSX）。
	ExportPdfRows int `json:",default=2000"`
	// PDF 中文字体（ADR-18 Go 原生渲染；空 = 禁 PDF 导出并报业务错误）。
	PdfFont string `json:",default=C:/Windows/Fonts/simhei.ttf"`
	// 对账差异站内信收件人（0=平台管理员 seed uid）。
	ReconTargetUID int64 `json:",default=0"`
	// 读模型重放起始时间提示（重建后 Kafka 已过保留期的历史需 p4backfill 补，仅日志）。
	ReplayHintDays int    `json:",default=7"`
	ConfigKey      string `json:",default=/micro/config/analytics/main"`
}
