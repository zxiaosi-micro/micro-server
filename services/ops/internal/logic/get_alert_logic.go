package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertLogic {
	return &GetAlertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAlertLogic) GetAlert(in *pb.GetAlertReq) (*pb.GetAlertResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	a, err := l.svcCtx.Models.Alert.FindOneScoped(l.ctx, tid, in.AlertId)
	if err != nil {
		return nil, errAlertNotFound
	}
	return &pb.GetAlertResp{Alert: pbAlert(a)}, nil
}
