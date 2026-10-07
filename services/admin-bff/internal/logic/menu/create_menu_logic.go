package menu

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateMenuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMenuLogic {
	return &CreateMenuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMenuLogic) CreateMenu(req *types.MenuCreateReq) (*types.SimpleResp, error) {
	resp, err := l.svcCtx.Identity.CreateMenu(l.ctx, &pb.CreateMenuReq{
		ParentId: parseID(req.ParentID), Name: req.Name, Type: int32(req.Type),
		PermCode: req.PermCode, Path: req.Path, Icon: req.Icon, Sort: int32(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(resp.MenuId, 10)}, nil
}
