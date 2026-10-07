package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleLogic {
	return &GetRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRole 角色详情 + 权限树回显。
func (l *GetRoleLogic) GetRole(in *pb.GetRoleReq) (*pb.GetRoleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	role, err := l.svcCtx.Models.Role.FindOne(l.ctx, tid, in.RoleId)
	if err != nil {
		return nil, errRoleNotFound
	}
	menuIds, err := l.svcCtx.Models.RoleMenu.FindMenuIdsByRole(l.ctx, tid, in.RoleId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.GetRoleResp{
		Role: &pb.RoleItem{
			RoleId:    role.RoleId,
			Code:      role.Code,
			Name:      role.Name,
			DataScope: role.DataScope.String,
			Remark:    role.Remark.String,
			CreatedAt: role.CreatedAt.UnixMilli(),
		},
		MenuIds: menuIds,
	}, nil
}
