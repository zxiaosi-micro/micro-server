package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type SetPriceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetPriceLogic {
	return &SetPriceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// SetPrice 设置价格：版本化留痕——插入 version+1 新行，历史不删改（01 FR-CTL-004）。
func (l *SetPriceLogic) SetPrice(in *pb.SetPriceReq) (*pb.SetPriceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.SkuId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("sku_id 必填")
	}
	switch in.PriceType {
	case "RETAIL", "DEALER", "TIER":
	default:
		return nil, errPriceTypeBad
	}
	if in.PriceType == "TIER" && in.TierQty <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("TIER 价格必须指定 tier_qty")
	}
	amount, err := parseAmount(in.Amount)
	if err != nil {
		return nil, errAmountBad.WithCause(err)
	}
	if _, err := l.svcCtx.Models.Sku.FindOne(l.ctx, tid, in.SkuId); err != nil {
		if err == model.ErrNotFound {
			return nil, errSkuNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}

	op := opUID(l.ctx)
	version, err := l.svcCtx.Models.Price.InsertLatest(l.ctx, &model.Price{
		PriceId:   l.svcCtx.Snowflake.MustNextID(),
		SkuId:     in.SkuId,
		PriceType: in.PriceType,
		TierQty:   int64(in.TierQty),
		Amount:    amount,
		TenantId:  tid,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.SetPriceResp{Version: int32(version)}, nil
}
