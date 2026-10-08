package logic

import (
	"context"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpsertDealerExtLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertDealerExtLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertDealerExtLogic {
	return &UpsertDealerExtLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpsertDealerExt 经销商扩展 1:1 幂等写入。
func (l *UpsertDealerExtLogic) UpsertDealerExt(in *pb.UpsertDealerExtReq) (*pb.UpsertDealerExtResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.PartyId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("party_id 必填")
	}
	if !validJSONString(in.RebateRule) {
		return nil, errcode.ErrBadRequest.WithMsg("rebate_rule 不是合法 JSON")
	}
	if _, err := l.svcCtx.Models.Party.FindOne(l.ctx, tid, in.PartyId); err != nil {
		return nil, partyErr(err)
	}

	level := in.DealerLevel
	if level == "" {
		level = "STANDARD"
	}
	err = l.svcCtx.Models.DealerExt.Upsert(l.ctx, &model.DealerExt{
		PartyId:          in.PartyId,
		DealerLevel:      level,
		AuthorizedRegion: toNullString(in.AuthorizedRegion),
		RebateRule:       toNullString(in.RebateRule),
		TenantId:         tid,
		CreatedBy:        toNullInt64(opUID(l.ctx)),
	}, opUID(l.ctx))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpsertDealerExtResp{}, nil
}
