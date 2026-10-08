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

type ListStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 员工列表(perm: party:party:list)
func NewListStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStaffLogic {
	return &ListStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListStaffLogic) ListStaff(req *types.IDPath) (resp *types.StaffListResp, err error) {
	r, err := l.svcCtx.Party.ListStaff(l.ctx, &partyPb.ListStaffReq{PartyId: parseID(req.ID), Page: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	resp = &types.StaffListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StaffItem{
			StaffID: strconv.FormatInt(s.StaffId, 10), PartyID: strconv.FormatInt(s.PartyId, 10),
			UserID: strconv.FormatInt(s.UserId, 10), Name: s.Name, StaffType: s.StaffType,
			SkillTags: parseStringSlice(s.SkillTags), WorkRegion: s.WorkRegion,
			Status: int(s.Status), CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}
