package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseAlertLogic {
	return &CloseAlertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CloseAlert 关闭（前置：closed_ticket_id 已验收工单，或人工备注原因）。
func (l *CloseAlertLogic) CloseAlert(in *pb.CloseAlertReq) (*pb.CloseAlertResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	alert, err := l.svcCtx.Models.Alert.FindOneScoped(l.ctx, tid, in.AlertId)
	if err != nil {
		return nil, errAlertNotFound
	}
	// 关闭前置：关联已验收工单 或 人工备注原因（FR-OPS-003）
	if in.TicketId > 0 {
		tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
		if err != nil {
			return nil, errTicketNotFound
		}
		if tk.Status != "VERIFIED" && tk.Status != "CLOSED" {
			return nil, errAlertCloseNeed
		}
	} else if in.Reason == "" {
		return nil, errAlertCloseNeed
	}
	if err := l.svcCtx.Models.Alert.UpdateClose(l.ctx, tid, in.AlertId, in.TicketId, in.Reason); err != nil {
		return nil, err
	}
	refreshAlertUnread(l.svcCtx, tid, alert.StationId)
	return &pb.CloseAlertResp{}, nil
}
