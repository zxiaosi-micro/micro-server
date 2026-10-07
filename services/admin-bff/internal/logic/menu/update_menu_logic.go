package menu

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMenuLogic {
	return &UpdateMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMenuLogic) UpdateMenu(req *types.MenuUpdateReq) (*types.SimpleResp, error) {
	mid, err := strconv.ParseInt(req.MenuID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("menu_id 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateMenu(l.ctx, &pb.UpdateMenuReq{
		MenuId: mid, ParentId: parseID(req.ParentID), Name: req.Name, Type: int32(req.Type),
		PermCode: req.PermCode, Path: req.Path, Icon: req.Icon, Sort: int32(req.Sort), Status: int32(req.Status),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.MenuID}, nil
}
