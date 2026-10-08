package logic

import (
	"context"

	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateStocktakeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStocktakeLogic {
	return &CreateStocktakeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateStocktake 创建盘点单：快照该仓库全部库存账面数为明细（book_qty）。
func (l *CreateStocktakeLogic) CreateStocktake(in *pb.CreateStocktakeReq) (*pb.CreateStocktakeResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.WarehouseId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("warehouse_id 必填")
	}
	if err := warehouseExists(l.ctx, l.svcCtx, tid, in.WarehouseId); err != nil {
		return nil, err
	}

	// 快照当前账面（可用口径）
	rows, _, err := l.svcCtx.Models.Inventory.ListPage(l.ctx, tid, in.WarehouseId, 0, false, 1, 100)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	stid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx,
			"insert into `stocktake` (`stocktake_id`, `warehouse_id`, `status`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, 1, ?, ?, ?, ?)",
			stid, in.WarehouseId, in.Remark, tid, toNullInt64(op), toNullInt64(op)); err != nil {
			return err
		}
		for _, inv := range rows {
			if _, err := session.ExecCtx(ctx,
				"insert into `stocktake_item` (`item_id`, `stocktake_id`, `sku_id`, `book_qty`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?)",
				l.svcCtx.Snowflake.MustNextID(), stid, inv.SkuId, inv.Available, tid, toNullInt64(op), toNullInt64(op)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateStocktakeResp{StocktakeId: stid}, nil
}
