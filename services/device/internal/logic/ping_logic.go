package logic

import (
	"context"

	"micro-server/services/device/internal/svc"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PingLogic {
	return &PingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *PingLogic) Ping(_ *pb.PingReq) (*pb.PingResp, error) {
	return &pb.PingResp{Pong: "pong"}, nil
}
