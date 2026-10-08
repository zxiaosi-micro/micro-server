package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ReleaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseLogic {
	return &ReleaseLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// Release 释放预留（locked→available，订单取消/支付超时，FR-INV-010）。
func (l *ReleaseLogic) Release(in *pb.ReleaseReq) (*pb.ReleaseResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.WarehouseId <= 0 || in.SkuId <= 0 || in.Qty <= 0 {
		return nil, errQtyBad
	}
	if in.BizNo == "" {
		return nil, errBizNoBad
	}

	recordId, err := applyStockTx(l.ctx, l.svcCtx, tid, opUID(l.ctx),
		in.WarehouseId, in.SkuId, "RELEASE", in.BizNo, "",
		func(ctx context.Context, session sqlx.Session, inv *model.Inventory) (int64, error) {
			n, err := l.svcCtx.Models.Inventory.ReleaseInTx(ctx, session, inv.InventoryId, int64(in.Qty))
			if err != nil {
				return 0, err
			}
			if n == 0 {
				return 0, ErrLockedInsufficient
			}
			inv.Available += int64(in.Qty)
			inv.Locked -= int64(in.Qty)
			return int64(in.Qty), nil
		}, nil)
	if err != nil {
		return nil, mapTxErr(err)
	}

	// Redis 计数器同步（释放归还可用量，best-effort）
	syncGateInc(l.ctx, l.svcCtx, tid, in.WarehouseId, in.SkuId, int64(in.Qty))
	return &pb.ReleaseResp{RecordId: recordId}, nil
}
