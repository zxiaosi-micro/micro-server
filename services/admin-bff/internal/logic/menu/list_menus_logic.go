package menu

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMenusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMenusLogic {
	return &ListMenusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMenus 菜单管理平铺（菜单管理页 + 角色授权树）。
func (l *ListMenusLogic) ListMenus() (*types.MenuListResp, error) {
	resp, err := l.svcCtx.Identity.ListMenus(l.ctx, &pb.ListMenusReq{})
	if err != nil {
		return nil, err
	}
	list := make([]types.MenuItem, 0, len(resp.List))
	for _, m := range resp.List {
		list = append(list, menuItem(m))
	}
	return &types.MenuListResp{List: list}, nil
}

func menuItem(m *pb.MenuItem) types.MenuItem {
	return types.MenuItem{
		MenuID:   strconv.FormatInt(m.MenuId, 10),
		ParentID: strconv.FormatInt(m.ParentId, 10),
		Name:     m.Name,
		Type:     int(m.Type),
		PermCode: m.PermCode,
		Path:     m.Path,
		Icon:     m.Icon,
		Sort:     int(m.Sort),
		Status:   int(m.Status),
	}
}
