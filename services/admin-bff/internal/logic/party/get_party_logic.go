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

type GetPartyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 参与方详情(perm: party:party:list)
func NewGetPartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPartyLogic {
	return &GetPartyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPartyLogic) GetParty(req *types.IDPath) (resp *types.PartyDetailResp, err error) {
	pid := parseID(req.ID)
	r, err := l.svcCtx.Party.GetParty(l.ctx, &partyPb.GetPartyReq{PartyId: pid})
	if err != nil {
		return nil, err
	}
	p := r.Party
	return &types.PartyDetailResp{Party: types.PartyItem{
		PartyID: strconv.FormatInt(p.PartyId, 10), Name: p.Name,
		Type: parseStringSlice(p.Type), Status: int(p.Status),
		CreditCode: p.CreditCode, Region: p.Region, Address: p.Address, Remark: p.Remark,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}}, nil
}
