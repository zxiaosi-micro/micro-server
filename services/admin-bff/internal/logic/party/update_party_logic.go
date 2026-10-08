// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePartyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 编辑参与方(perm: party:party:update)
func NewUpdatePartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePartyLogic {
	return &UpdatePartyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePartyLogic) UpdateParty(req *types.PartyUpdateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpdateParty(l.ctx, &partyPb.UpdatePartyReq{
		PartyId: parseID(req.Id), Name: req.Name, Type: req.Type, Status: int32(req.Status),
		CreditCode: req.CreditCode, Region: req.Region, Address: req.Address, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
