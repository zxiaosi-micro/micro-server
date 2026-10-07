package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListMenusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenusLogic {
	return &ListMenusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMenus 租户全量菜单平铺（菜单管理页 + 角色授权树）。
func (l *ListMenusLogic) ListMenus(in *pb.ListMenusReq) (*pb.ListMenusResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Menu.FindAllByTenant(l.ctx, tid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.MenuItem, 0, len(list))
	for _, m := range list {
		out = append(out, &pb.MenuItem{
			MenuId:   m.MenuId,
			ParentId: m.ParentId,
			Name:     m.Name,
			Type:     int32(m.Type),
			PermCode: m.PermCode.String,
			Path:     m.Path.String,
			Icon:     m.Icon.String,
			Sort:     int32(m.Sort),
			Status:   int32(m.Status),
		})
	}
	return &pb.ListMenusResp{List: out}, nil
}
