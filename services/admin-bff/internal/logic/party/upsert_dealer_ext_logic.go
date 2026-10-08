// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertDealerExtLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 经销商扩展 Upsert(perm: party:dealer:update)
func NewUpsertDealerExtLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertDealerExtLogic {
	return &UpsertDealerExtLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertDealerExtLogic) UpsertDealerExt(req *types.DealerExtUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpsertDealerExt(l.ctx, &partyPb.UpsertDealerExtReq{
		PartyId: parseID(req.Id), DealerLevel: req.DealerLevel,
		AuthorizedRegion: req.AuthorizedRegion, RebateRule: req.RebateRule,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
