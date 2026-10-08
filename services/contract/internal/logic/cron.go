// 质保到期扫描（cron 注册见 contract.go；E16：扫描 SQL 命中 idx_warranty_expiry_scan 索引）。

package logic

import (
	"context"
	"time"

	"micro-server/services/contract/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ScanWarrantyExpiry 到期扫描：ACTIVE 且 end_at < now → EXPIRED。
func ScanWarrantyExpiry(ctx context.Context, sc *svc.ServiceContext) error {
	ctx = tenantSkip(ctx)
	expired, err := sc.Models.Warranty.FindExpired(ctx, time.Now(), 500)
	if err != nil {
		return err
	}
	for _, w := range expired {
		err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.Warranty.MarkExpiredTx(ctx, session, w.TenantId, w.WarrantyId)
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("质保到期置位失败 warranty_no=%s: %v", w.WarrantyNo, err)
			continue
		}
		logx.WithContext(ctx).Infof("质保已到期 warranty_no=%s target=%s", w.WarrantyNo, w.TargetKey)
	}
	return nil
}
