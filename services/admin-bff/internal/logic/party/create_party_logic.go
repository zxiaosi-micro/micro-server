// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePartyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建参与方(perm: party:party:create)
func NewCreatePartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePartyLogic {
	return &CreatePartyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePartyLogic) CreateParty(req *types.PartyCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateParty(l.ctx, &partyPb.CreatePartyReq{
		Name: req.Name, Type: req.Type, CreditCode: req.CreditCode,
		Region: req.Region, Address: req.Address, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
