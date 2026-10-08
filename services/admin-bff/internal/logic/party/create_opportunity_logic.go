// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOpportunityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建商机(perm: party:opportunity:create)
func NewCreateOpportunityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOpportunityLogic {
	return &CreateOpportunityLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateOpportunityLogic) CreateOpportunity(req *types.OpportunityCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateOpportunity(l.ctx, &partyPb.CreateOpportunityReq{
		PartyId: parseID(req.PartyID), Title: req.Title, Amount: req.Amount,
		ExpectedCloseDate: req.ExpectedClose, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
