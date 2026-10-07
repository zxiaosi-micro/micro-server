package auth

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type StepUpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStepUpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepUpLogic {
	return &StepUpLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// StepUp 二次认证提级：password + ctx 中的 sid（BearerProbe 注入）→
// identity 校验密码并重签 level=2 access（5min 窗口）。
func (l *StepUpLogic) StepUp(req *types.StepUpReq) (*types.StepUpResp, error) {
	sid := ctxkit.SID(l.ctx)
	if sid == "" {
		return nil, errcode.ErrTokenInvalid
	}
	resp, err := l.svcCtx.Identity.StepUp(l.ctx, &pb.StepUpReq{
		Sid:      sid,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &types.StepUpResp{
		Level:           int(resp.Level),
		AccessToken:     resp.AccessToken,
		AccessExpiresIn: resp.AccessExpiresIn,
	}, nil
}
