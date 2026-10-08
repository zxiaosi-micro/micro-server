package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SpareOutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSpareOutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SpareOutLogic {
	return &SpareOutLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// SpareOut 备件领用（FR-INV-008：必须关联工单号；biz_no 即工单号，幂等防重复领用）。
func (l *SpareOutLogic) SpareOut(in *pb.SpareOutReq) (*pb.SpareOutResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.WarehouseId <= 0 || in.SkuId <= 0 || in.Qty <= 0 {
		return nil, errQtyBad
	}
	if in.WorkOrderNo == "" {
		return nil, errWorkOrderRequired
	}

	recordId, err := applyStockTx(l.ctx, l.svcCtx, tid, opUID(l.ctx),
		in.WarehouseId, in.SkuId, "SPARE_OUT", in.WorkOrderNo, in.Remark,
		func(ctx context.Context, session sqlx.Session, inv *model.Inventory) (int64, error) {
			n, err := l.svcCtx.Models.Inventory.AdjustAvailableInTx(ctx, session, inv.InventoryId, -int64(in.Qty))
			if err != nil {
				return 0, err
			}
			if n == 0 {
				return 0, ErrInsufficientStock
			}
			inv.Available -= int64(in.Qty)
			return -int64(in.Qty), nil
		}, nil)
	if err != nil {
		return nil, mapTxErr(err)
	}

	// 以 DB 为准校正计数器（低频操作，读回 after 值直接 SET）
	if inv, ierr := l.svcCtx.Models.Inventory.FindOne(l.ctx, tid, in.WarehouseId, in.SkuId); ierr == nil {
		syncGateSet(l.ctx, l.svcCtx, tid, in.WarehouseId, in.SkuId, inv.Available)
	}
	return &pb.SpareOutResp{RecordId: recordId}, nil
}
