package user

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserLogic) GetUser(req *types.IDPath) (*types.UserItem, error) {
	uid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
	}
	resp, err := l.svcCtx.Identity.GetUser(l.ctx, &pb.GetUserReq{Uid: uid})
	if err != nil {
		return nil, err
	}
	out := userItem(resp.User)
	// 编辑页回显需要绑定角色 ID（详情走 identity 二次查询由 user 逻辑聚合）
	if len(out.RoleCodes) == 0 {
		out.RoleCodes = []string{}
	}
	return &out, nil
}
