package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRoleLogic {
	return &DeleteRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteRole 软删：有用户绑定拒绝 → 清 role_menu。
func (l *DeleteRoleLogic) DeleteRole(in *pb.DeleteRoleReq) (*pb.DeleteRoleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.RoleId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("role_id 必填")
	}
	if _, err := l.svcCtx.Models.Role.FindOne(l.ctx, tid, in.RoleId); err != nil {
		return nil, errRoleNotFound
	}
	uids, err := l.svcCtx.Models.UserRole.FindUserIdsByRole(l.ctx, tid, in.RoleId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if len(uids) > 0 {
		return nil, errRoleHasUsers
	}
	if err := l.svcCtx.Models.Role.SoftDelete(l.ctx, tid, in.RoleId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	_ = l.svcCtx.Models.RoleMenu.DeleteByRole(l.ctx, tid, in.RoleId)
	return &pb.DeleteRoleResp{}, nil
}
