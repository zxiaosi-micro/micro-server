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

type UpdateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserLogic {
	return &UpdateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateUserLogic) UpdateUser(req *types.UserUpdateReq) (*types.SimpleResp, error) {
	uid, err := strconv.ParseInt(req.UID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateUser(l.ctx, &pb.UpdateUserReq{
		Uid:      uid,
		Nickname: req.Nickname,
		Mobile:   req.Mobile,
		Email:    req.Email,
		Types:    req.Types,
		OrgId:    parseID(req.OrgID),
		RoleIds:  parseIDs(req.RoleIDs),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.UID}, nil
}
