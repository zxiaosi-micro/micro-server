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

type ListPartiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 参与方列表(perm: party:party:list)
func NewListPartiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPartiesLogic {
	return &ListPartiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPartiesLogic) ListParties(req *types.PartyListReq) (resp *types.PartyListResp, err error) {
	r, err := l.svcCtx.Party.ListParty(l.ctx, &partyPb.ListPartyReq{
		Keyword: req.Keyword, Type: req.Type, Status: int32(req.Status),
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.PartyListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.PartyItem{
			PartyID: strconv.FormatInt(p.PartyId, 10), Name: p.Name,
			Type: parseStringSlice(p.Type), Status: int(p.Status),
			CreditCode: p.CreditCode, Region: p.Region, Address: p.Address, Remark: p.Remark,
			CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		})
	}
	return resp, nil
}
