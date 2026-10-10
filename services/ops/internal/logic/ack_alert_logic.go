package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type AckAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAckAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AckAlertLogic {
	return &AckAlertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AckAlert 确认（OPEN→ACKED，记录确认人）。
func (l *AckAlertLogic) AckAlert(in *pb.AckAlertReq) (*pb.AckAlertResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	if err := l.svcCtx.Models.Alert.UpdateAck(l.ctx, tid, in.AlertId, uid); err != nil {
		return nil, err
	}
	// 未读计数对账（确认后重算回填）
	if a, err := l.svcCtx.Models.Alert.FindOneScoped(l.ctx, tid, in.AlertId); err == nil {
		refreshAlertUnread(l.svcCtx, tid, a.StationId)
	}
	return &pb.AckAlertResp{}, nil
}
