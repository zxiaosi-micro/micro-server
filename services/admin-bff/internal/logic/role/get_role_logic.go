package role

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRoleLogic {
	return &GetRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetRole 角色详情 + 权限树回显。
func (l *GetRoleLogic) GetRole(req *types.IDPath) (*types.RoleDetailResp, error) {
	rid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("role_id 非法")
	}
	resp, err := l.svcCtx.Identity.GetRole(l.ctx, &pb.GetRoleReq{RoleId: rid})
	if err != nil {
		return nil, err
	}
	menuIDs := make([]string, 0, len(resp.MenuIds))
	for _, id := range resp.MenuIds {
		menuIDs = append(menuIDs, strconv.FormatInt(id, 10))
	}
	return &types.RoleDetailResp{Role: roleItem(resp.Role), MenuIDs: menuIDs}, nil
}
