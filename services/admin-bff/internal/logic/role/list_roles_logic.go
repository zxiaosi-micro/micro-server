package role

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRolesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRolesLogic {
	return &ListRolesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRolesLogic) ListRoles(req *types.RoleListReq) (*types.RoleListResp, error) {
	resp, err := l.svcCtx.Identity.ListRoles(l.ctx, &pb.ListRolesReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	list := make([]types.RoleItem, 0, len(resp.List))
	for _, r := range resp.List {
		list = append(list, roleItem(r))
	}
	return &types.RoleListResp{List: list, Total: resp.Total}, nil
}

func roleItem(r *pb.RoleItem) types.RoleItem {
	item := types.RoleItem{
		RoleID:    strconv.FormatInt(r.RoleId, 10),
		Code:      r.Code,
		Name:      r.Name,
		DataScope: r.DataScope,
		Remark:    r.Remark,
	}
	if r.CreatedAt > 0 {
		item.CreatedAt = strconv.FormatInt(r.CreatedAt, 10)
	}
	return item
}
