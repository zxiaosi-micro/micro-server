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

type ListOpportunitiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商机列表(perm: party:opportunity:list)
func NewListOpportunitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOpportunitiesLogic {
	return &ListOpportunitiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListOpportunitiesLogic) ListOpportunities(req *types.OpportunityListReq) (resp *types.OpportunityListResp, err error) {
	r, err := l.svcCtx.Party.ListOpportunity(l.ctx, &partyPb.ListOpportunityReq{
		PartyId: parseID(req.PartyID), Stage: req.Stage, Keyword: req.Keyword,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.OpportunityListResp{Total: r.Total}
	for _, o := range r.List {
		resp.List = append(resp.List, types.OpportunityItem{
			OpportunityID: strconv.FormatInt(o.OpportunityId, 10), PartyID: strconv.FormatInt(o.PartyId, 10),
			Title: o.Title, Stage: o.Stage, Amount: o.Amount,
			ExpectedClose: o.ExpectedCloseDate, Remark: o.Remark,
			CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
		})
	}
	return resp, nil
}
