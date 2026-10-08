package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetDealerExtLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDealerExtLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDealerExtLogic {
	return &GetDealerExtLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetDealerExt 取经销商扩展；未建档返回空 item（party_id=0），前端按需引导建档。
func (l *GetDealerExtLogic) GetDealerExt(in *pb.GetDealerExtReq) (*pb.GetDealerExtResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	d, err := l.svcCtx.Models.DealerExt.FindOne(l.ctx, tid, in.PartyId)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return &pb.GetDealerExtResp{DealerExt: &pb.DealerExtItem{}}, nil
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.GetDealerExtResp{DealerExt: &pb.DealerExtItem{
		PartyId:          d.PartyId,
		DealerLevel:      d.DealerLevel,
		AuthorizedRegion: d.AuthorizedRegion.String,
		RebateRule:       d.RebateRule.String,
		UpdatedAt:        d.UpdatedAt.UnixMilli(),
	}}, nil
}
