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

type StockInLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStockInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StockInLogic {
	return &StockInLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// StockIn 入库（采购/退货/调拨，FR-INV-003）：建档或累加 + 流水 + Redis 同步。
func (l *StockInLogic) StockIn(in *pb.StockInReq) (*pb.StockInResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.WarehouseId <= 0 || in.SkuId <= 0 || in.Qty <= 0 {
		return nil, errQtyBad
	}
	switch in.BizType {
	case "STOCK_IN", "RETURN_IN", "TRANSFER_IN":
	default:
		return nil, errBizTypeBad
	}
	if in.BizNo == "" {
		return nil, errBizNoBad
	}
	if len(in.Sns) > 0 && len(in.Sns) != int(in.Qty) {
		return nil, errSnQtyMismatch
	}
	if err := warehouseExists(l.ctx, l.svcCtx, tid, in.WarehouseId); err != nil {
		return nil, err
	}

	op := opUID(l.ctx)
	var recordId int64
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 建档或累加（uk_inventory_wh_sku 冲突走 UPDATE 累加）
		if err := l.svcCtx.Models.Inventory.InitRowOnDupInTx(ctx, session, &model.Inventory{
			InventoryId: l.svcCtx.Snowflake.MustNextID(),
			WarehouseId: in.WarehouseId,
			SkuId:       in.SkuId,
			Available:   int64(in.Qty),
			TenantId:    tid,
			CreatedBy:   toNullInt64(op),
			UpdatedBy:   toNullInt64(op),
		}); err != nil {
			return err
		}
		inv, err := l.svcCtx.Models.Inventory.LockByWhSku(ctx, session, tid, in.WarehouseId, in.SkuId)
		if err != nil {
			return err
		}
		before := inv.Available - int64(in.Qty)

		recordId = l.svcCtx.Snowflake.MustNextID()
		if err := l.svcCtx.Models.StockRecord.InsertInTx(ctx, session, &model.StockRecord{
			RecordId:        recordId,
			InventoryId:     inv.InventoryId,
			WarehouseId:     in.WarehouseId,
			SkuId:           in.SkuId,
			BizType:         in.BizType,
			BizNo:           in.BizNo,
			Qty:             int64(in.Qty),
			BeforeAvailable: before,
			AfterAvailable:  inv.Available,
			BeforeLocked:    inv.Locked,
			AfterLocked:     inv.Locked,
			Remark:          toNullString(in.Remark),
			TenantId:        tid,
			CreatedBy:       toNullInt64(op),
			UpdatedBy:       toNullInt64(op),
		}); err != nil {
			if isDupKey(err) {
				return ErrTxnReplay
			}
			return err
		}
		// stock_in 事件（S6-01：device 消费驱动设备状态机 IN_STOCK；与流水同事务 Outbox）
		if err := eventbus.Emit(ctx, session, stockInEvent(tid, in.WarehouseId, in.SkuId, int64(in.Qty), in.BizNo, in.Sns)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, mapTxErr(err)
	}

	// Redis 计数器同步（入库方向增量，best-effort；对账 cron 兜底）
	syncGateInc(l.ctx, l.svcCtx, tid, in.WarehouseId, in.SkuId, int64(in.Qty))
	return &pb.StockInResp{RecordId: recordId}, nil
}
