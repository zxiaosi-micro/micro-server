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

type ResetUserPasswordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetUserPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserPasswordLogic {
	return &ResetUserPasswordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ResetUserPassword 重置密码（identity 落 argon2id + 全端注销 + 审计）。
func (l *ResetUserPasswordLogic) ResetUserPassword(req *types.UserPasswordReq) (*types.SimpleResp, error) {
	uid, err := strconv.ParseInt(req.UID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
	}
	if _, err := l.svcCtx.Identity.ResetPassword(l.ctx, &pb.ResetPasswordReq{
		Uid: uid, NewPassword: req.NewPassword,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.UID}, nil
}
