package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateStationProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStationProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStationProductLogic {
	return &CreateStationProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateStationProduct 新建场站模板：头 + BOM 明细同事务落库。
func (l *CreateStationProductLogic) CreateStationProduct(in *pb.CreateStationProductReq) (*pb.CreateStationProductResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("name 必填")
	}
	if len(in.Items) == 0 {
		return nil, errBomEmpty
	}
	// BOM 明细 SKU 存在性校验（同库，避免事务内半途失败）
	for _, it := range in.Items {
		if it.SkuId <= 0 || it.Qty <= 0 {
			return nil, errcode.ErrBadRequest.WithMsg("BOM 明细 sku_id/qty 必须大于 0")
		}
		if _, err := l.svcCtx.Models.Sku.FindOne(l.ctx, tid, it.SkuId); err != nil {
			if err == model.ErrNotFound {
				return nil, errBomSkuNotFound
			}
			return nil, errcode.Internal.WithCause(err)
		}
	}

	spid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx,
			"insert into `station_product` (`station_product_id`, `name`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?)",
			spid, in.Name, in.Remark, tid, toNullInt64(op), toNullInt64(op)); err != nil {
			return err
		}
		for _, it := range in.Items {
			if _, err := session.ExecCtx(ctx,
				"insert into `station_product_item` (`item_id`, `station_product_id`, `sku_id`, `qty`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?)",
				l.svcCtx.Snowflake.MustNextID(), spid, it.SkuId, it.Qty, tid, toNullInt64(op), toNullInt64(op)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateStationProductResp{StationProductId: spid}, nil
}
