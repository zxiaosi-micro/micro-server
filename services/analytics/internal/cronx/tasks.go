// tasks.go · analytics cron 扫描器（ADR-09；cron 注册表登记 + last_run 对账）。
package cronx

import (
	"context"
	"time"

	"micro-server/services/analytics/internal/confcenter"
	"micro-server/services/analytics/internal/logic"
	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// RegisterAll 登记全部扫描任务。
func RegisterAll(sc *svc.ServiceContext) {
	Register(Task{
		Name:     "daily_recon",
		Interval: time.Minute,
		Run: func(ctx context.Context) error {
			// 对账 cron 由 configcenter 开关控制（生产默认开，dev 可关）；差异走 recon_diff + 站内信
			if !confcenter.Current().ReconEnabled {
				return nil
			}
			day := logic.StatDateOf(time.Now().AddDate(0, 0, -1))
			diff, err := logic.RunReconCore(ctx, sc, day)
			if err != nil {
				return err
			}
			if diff > 0 {
				logx.Errorf("analytics: daily_recon 差异 %d（biz_date=%s，站内信已发）", diff, day.Format("2006-01-02"))
			}
			return nil
		},
	})
	Register(Task{
		Name:     "dw_etl_daily",
		Interval: time.Hour,
		Run: func(ctx context.Context) error {
			// 数仓日调度（S10-02）：昨日批次缺失即补跑（服务重启自愈；ODKU/IGNORE 幂等）。
			// K8s CronJob 候选形态（ADR-09 阶段二）：届时 CronJob 调 RunDwEtl RPC，本任务退化为兜底。
			return logic.RunDwEtlMissing(ctx, sc)
		},
	})
}
