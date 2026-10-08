package logic

import (
	"context"

	"micro-server/services/inventory/internal/confcenter"
	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

type ReserveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReserveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReserveLogic {
	return &ReserveLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// Reserve 预留（available→locked）——防超卖双保险主路径（02 §9.2）。
func (l *ReserveLogic) Reserve(in *pb.ReserveReq) (*pb.ReserveResp, error) {
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

	// 1. Redis DECRBY 原子预扣（挡并发）。三态：拒绝直接短路返回；
	//    降级（key 缺失/故障/开关关闭）走纯 DB（第 4 步口径）；放行走 DB，失败须归还。
	gate := gateDegrade
	if confcenter.Current().RedisGateEnabled {
		gate = stockGate(l.ctx, l.svcCtx, tid, in.WarehouseId, in.SkuId, int64(in.Qty))
	}
	switch gate {
	case gateReject:
		return nil, ErrInsufficientStock
	case gateDegrade:
		l.Infof("reserve 降级纯 DB 路径 wid=%d sku=%d qty=%d", in.WarehouseId, in.SkuId, in.Qty)
	}

	// 2+3. DB 事务：行锁 + 余量守卫 UPDATE → 流水（1062 → ErrTxnReplay）→ Outbox（低库存事件）。
	recordId, err := applyStockTx(l.ctx, l.svcCtx, tid, opUID(l.ctx),
		in.WarehouseId, in.SkuId, "RESERVE", in.BizNo, "",
		func(ctx context.Context, session sqlx.Session, inv *model.Inventory) (int64, error) {
			n, err := l.svcCtx.Models.Inventory.ReserveInTx(ctx, session, inv.InventoryId, int64(in.Qty))
			if err != nil {
				return 0, err
			}
			if n == 0 {
				return 0, ErrInsufficientStock
			}
			inv.Available -= int64(in.Qty)
			inv.Locked += int64(in.Qty)
			return int64(in.Qty), nil
		},
		func(before, after *model.Inventory) []eventbus.EmitInput {
			return stockLowEvents(tid, in.WarehouseId, in.SkuId, before, after)
		})
	if err != nil {
		if gate == gatePass {
			rollbackGate(l.ctx, l.svcCtx, tid, in.WarehouseId, in.SkuId, int64(in.Qty))
		}
		return nil, mapTxErr(err)
	}
	return &pb.ReserveResp{RecordId: recordId}, nil
}
