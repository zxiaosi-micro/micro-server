package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuLogic {
	return &DeleteMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteMenu 软删：有子节点拒绝；清 role_menu 绑定 + 相关用户 auth_cache 刷新。
func (l *DeleteMenuLogic) DeleteMenu(in *pb.DeleteMenuReq) (*pb.DeleteMenuResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.MenuId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("menu_id 必填")
	}
	if _, err := l.svcCtx.Models.Menu.FindOne(l.ctx, tid, in.MenuId); err != nil {
		return nil, errMenuNotFound
	}
	children, err := l.svcCtx.Models.Menu.CountChildren(l.ctx, tid, in.MenuId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if children > 0 {
		return nil, errHasChildren
	}
	if err := l.svcCtx.Models.Menu.SoftDelete(l.ctx, tid, in.MenuId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	_ = l.svcCtx.Models.RoleMenu.DeleteByMenu(l.ctx, tid, in.MenuId)
	invalidateTenantAuthCache(l.svcCtx, tid)
	return &pb.DeleteMenuResp{}, nil
}
