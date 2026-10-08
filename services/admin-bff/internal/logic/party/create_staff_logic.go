// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建员工(perm: party:staff:create)
func NewCreateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStaffLogic {
	return &CreateStaffLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateStaffLogic) CreateStaff(req *types.StaffCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.CreateStaff(l.ctx, &partyPb.CreateStaffReq{
		PartyId: parseID(req.PartyID), UserId: parseID(req.UserID), Name: req.Name,
		StaffType: req.StaffType, SkillTags: req.SkillTags, WorkRegion: req.WorkRegion,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
