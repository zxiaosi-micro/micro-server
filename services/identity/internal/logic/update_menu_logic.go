package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuLogic {
	return &UpdateMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateMenu 编辑菜单（perm_code 变更后受影响用户 auth_cache 刷新）。
func (l *UpdateMenuLogic) UpdateMenu(in *pb.UpdateMenuReq) (*pb.UpdateMenuResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.MenuId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("menu_id 必填")
	}
	cur, err := l.svcCtx.Models.Menu.FindOne(l.ctx, tid, in.MenuId)
	if err != nil {
		return nil, errMenuNotFound
	}
	permCode := cur.PermCode.String
	if in.PermCode != "" {
		permCode = in.PermCode
	}
	if err := l.svcCtx.Models.Menu.Update(l.ctx, tid, in.MenuId, in.ParentId, in.Name, int64(in.Type),
		permCode, in.Path, in.Icon, int64(in.Sort), int64(in.Status), opUID(l.ctx)); err != nil {
		if isDupKey(err) {
			return nil, errPermCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateMenuResp{}, nil
}
