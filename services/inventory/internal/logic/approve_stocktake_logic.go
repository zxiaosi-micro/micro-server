package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ApproveStocktakeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveStocktakeLogic {
	return &ApproveStocktakeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ApproveStocktake 盘点审批：通过 → 按差异调整账面（STOCKTAKE_ADJUST 流水，同事务）；
// 驳回 → 状态回草稿（差异留痕，不改账面）。
func (l *ApproveStocktakeLogic) ApproveStocktake(in *pb.ApproveStocktakeReq) (*pb.ApproveStocktakeResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	st, err := l.svcCtx.Models.Stocktake.FindOne(l.ctx, tid, in.StocktakeId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStocktakeNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	if st.Status != 2 {
		return nil, errStocktakeStatus
	}

	op := opUID(l.ctx)
	newStatus := int64(3)
	if !in.Approve {
		newStatus = 4
	}

	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		items, err := l.svcCtx.Models.StocktakeItem.ListByStocktake(ctx, tid, in.StocktakeId)
		if err != nil {
			return err
		}
		for _, it := range items {
			if !it.CountedQty.Valid {
				continue // 未盘项跳过
			}
			diff := it.CountedQty.Int64 - it.BookQty
			if err := l.svcCtx.Models.StocktakeItem.MarkDiff(ctx, session, it.ItemId, diff); err != nil {
				return err
			}
			if !in.Approve || diff == 0 {
				continue
			}
			// 账面调整（有流水；以行锁保并发安全）
			inv, err := l.svcCtx.Models.Inventory.LockByWhSku(ctx, session, tid, st.WarehouseId, it.SkuId)
			if err != nil {
				if err == model.ErrNotFound && diff > 0 {
					// 账面无行但实盘有货：建档入库
					if err := l.svcCtx.Models.Inventory.InitRowOnDupInTx(ctx, session, &model.Inventory{
						InventoryId: l.svcCtx.Snowflake.MustNextID(),
						WarehouseId: st.WarehouseId,
						SkuId:       it.SkuId,
						Available:   diff,
						TenantId:    tid,
						CreatedBy:   toNullInt64(op),
						UpdatedBy:   toNullInt64(op),
					}); err != nil {
						return err
					}
				} else if err != nil {
					return err
				} else {
					continue // 账面无行且实盘为零，无差异可调
				}
			} else {
				before := *inv
				if _, err := l.svcCtx.Models.Inventory.AdjustAvailableInTx(ctx, session, inv.InventoryId, diff); err != nil {
					return err
				}
				inv.Available += diff
				// 盘点调整流水（幂等键 = 盘点单号:SKU）
				if err := l.svcCtx.Models.StockRecord.InsertInTx(ctx, session, &model.StockRecord{
					RecordId:        l.svcCtx.Snowflake.MustNextID(),
					InventoryId:     inv.InventoryId,
					WarehouseId:     st.WarehouseId,
					SkuId:           it.SkuId,
					BizType:         "STOCKTAKE_ADJUST",
					BizNo:           "ST" + itoa(in.StocktakeId) + ":" + itoa(it.SkuId),
					Qty:             diff,
					BeforeAvailable: before.Available,
					AfterAvailable:  inv.Available,
					BeforeLocked:    inv.Locked,
					AfterLocked:     inv.Locked,
					Remark:          toNullString("盘点单 " + itoa(in.StocktakeId)),
					TenantId:        tid,
					CreatedBy:       toNullInt64(op),
					UpdatedBy:       toNullInt64(op),
				}); err != nil {
					return err
				}
				// 计数器校正事件攒批：事务外统一 SET（下方读回）
			}
		}
		return l.svcCtx.Models.Stocktake.UpdateStatus(ctx, tid, in.StocktakeId, newStatus, in.Remark, op)
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	// 审批通过后对涉及仓库 SKU 的计数器做一次权威校正（读回 DB 值 SET）
	if in.Approve {
		l.reconcileWarehouseGate(tid, st.WarehouseId)
	}
	return &pb.ApproveStocktakeResp{}, nil
}

// reconcileWarehouseGate 审批后校正该仓全部计数器（低频操作，逐行 best-effort）。
func (l *ApproveStocktakeLogic) reconcileWarehouseGate(tid, wid int64) {
	rows, _, err := l.svcCtx.Models.Inventory.ListPage(l.ctx, tid, wid, 0, false, 1, 100)
	if err != nil {
		return
	}
	for _, inv := range rows {
		syncGateSet(l.ctx, l.svcCtx, tid, wid, inv.SkuId, inv.Available)
	}
}
