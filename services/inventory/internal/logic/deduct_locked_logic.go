package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

type DeductLockedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeductLockedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeductLockedLogic {
	return &DeductLockedLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// DeductLocked 出库发货（locked 扣减，FR-INV-004：所有出库统一由 inventory 执行）。
// Redis 计数器（可用量口径）不受影响——预留时已扣过。
func (l *DeductLockedLogic) DeductLocked(in *pb.DeductLockedReq) (*pb.DeductLockedResp, error) {
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
		in.WarehouseId, in.SkuId, "DEDUCT", in.BizNo, in.Remark,
		func(ctx context.Context, session sqlx.Session, inv *model.Inventory) (int64, error) {
			n, err := l.svcCtx.Models.Inventory.DeductLockedInTx(ctx, session, inv.InventoryId, int64(in.Qty))
			if err != nil {
				return 0, err
			}
			if n == 0 {
				return 0, ErrLockedInsufficient
			}
			inv.Locked -= int64(in.Qty)
			return int64(in.Qty), nil
		},
		func(before, after *model.Inventory) []eventbus.EmitInput {
			return []eventbus.EmitInput{stockOutEvent(tid, in.WarehouseId, in.SkuId, int64(in.Qty), in.BizNo)}
		})
	if err != nil {
		return nil, mapTxErr(err)
	}
	return &pb.DeductLockedResp{RecordId: recordId}, nil
}
