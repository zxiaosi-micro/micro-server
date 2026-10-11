package logic

import (
	"context"

	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunDwEtlLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRunDwEtlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunDwEtlLogic {
	return &RunDwEtlLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}
