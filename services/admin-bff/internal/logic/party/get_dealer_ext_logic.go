// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDealerExtLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 经销商扩展(perm: party:party:list)
func NewGetDealerExtLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDealerExtLogic {
	return &GetDealerExtLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDealerExtLogic) GetDealerExt(req *types.IDPath) (resp *types.DealerExtResp, err error) {
	r, err := l.svcCtx.Party.GetDealerExt(l.ctx, &partyPb.GetDealerExtReq{PartyId: parseID(req.ID)})
	if err != nil {
		return nil, err
	}
	d := r.DealerExt
	return &types.DealerExtResp{DealerExt: types.DealerExtItem{
		PartyID: strconv.FormatInt(d.PartyId, 10), DealerLevel: d.DealerLevel,
		AuthorizedRegion: d.AuthorizedRegion, RebateRule: d.RebateRule, UpdatedAt: d.UpdatedAt,
	}}, nil
}
