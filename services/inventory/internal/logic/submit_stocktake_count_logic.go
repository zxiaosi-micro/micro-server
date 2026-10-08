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

type SubmitStocktakeCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitStocktakeCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitStocktakeCountLogic {
	return &SubmitStocktakeCountLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// SubmitStocktakeCount 提交实盘数（DRAFT/SUBMITTED → SUBMITTED；可重复提交覆盖）。
func (l *SubmitStocktakeCountLogic) SubmitStocktakeCount(in *pb.SubmitStocktakeCountReq) (*pb.SubmitStocktakeCountResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.StocktakeId <= 0 || len(in.Items) == 0 {
		return nil, errcode.ErrBadRequest.WithMsg("stocktake_id/items 必填")
	}
	for _, it := range in.Items {
		if it.SkuId <= 0 || it.CountedQty < 0 {
			return nil, errCountedBad
		}
	}
	st, err := l.svcCtx.Models.Stocktake.FindOne(l.ctx, tid, in.StocktakeId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStocktakeNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	if st.Status == 3 || st.Status == 4 {
		return nil, errStocktakeStatus
	}

	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, it := range in.Items {
			if err := l.svcCtx.Models.StocktakeItem.SubmitCount(ctx, session, in.StocktakeId, it.SkuId, int64(it.CountedQty)); err != nil {
				return err
			}
		}
		if err := l.svcCtx.Models.Stocktake.UpdateStatus(ctx, tid, in.StocktakeId, 2, "", op); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.SubmitStocktakeCountResp{}, nil
}
