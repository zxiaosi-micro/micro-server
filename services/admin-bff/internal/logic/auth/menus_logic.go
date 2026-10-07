package auth

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type MenusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MenusLogic {
	return &MenusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Menus 用户可见菜单（AppLayout 数据源）+ 按钮权限码（permission.ts 消费）。
func (l *MenusLogic) Menus() (*types.MenusResp, error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	resp, err := l.svcCtx.Identity.GetUserMenus(l.ctx, &pb.GetUserMenusReq{Uid: uid})
	if err != nil {
		return nil, err
	}
	menus := make([]types.MenuItem, 0, len(resp.Menus))
	for _, m := range resp.Menus {
		menus = append(menus, types.MenuItem{
			MenuID:   strconv.FormatInt(m.MenuId, 10),
			ParentID: strconv.FormatInt(m.ParentId, 10),
			Name:     m.Name,
			Type:     int(m.Type),
			PermCode: m.PermCode,
			Path:     m.Path,
			Icon:     m.Icon,
			Sort:     int(m.Sort),
			Status:   int(m.Status),
		})
	}
	perms := resp.Perms
	if perms == nil {
		perms = []string{}
	}
	return &types.MenusResp{Menus: menus, Perms: perms}, nil
}
