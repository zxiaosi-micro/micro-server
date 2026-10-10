package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
)

type TransferAlertToTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferAlertToTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferAlertToTicketLogic {
	return &TransferAlertToTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TransferAlertToTicket 转工单（隐含确认 OPEN→ACKED；level→优先级；可选自动派单）。
func (l *TransferAlertToTicketLogic) TransferAlertToTicket(in *pb.TransferAlertToTicketReq) (*pb.TransferAlertToTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	alert, err := l.svcCtx.Models.Alert.FindOneScoped(l.ctx, tid, in.AlertId)
	if err != nil {
		return nil, errAlertNotFound
	}
	if alert.Status == "CLOSED" || alert.Status == "ARCHIVED" {
		return nil, errAlertStatusBad
	}
	// 隐含确认：OPEN → ACKED
	if alert.Status == "OPEN" {
		if err := l.svcCtx.Models.Alert.UpdateAck(l.ctx, tid, in.AlertId, uid); err != nil {
			return nil, err
		}
		refreshAlertUnread(l.svcCtx, tid, alert.StationId)
	}
	// level → 优先级直映射（FR-OPS-004）
	tk := &model.Ticket{
		TicketId:    l.svcCtx.Snowflake.MustNextID(),
		Type:        "ALERT",
		Title:       "告警处理：" + alert.Title,
		Content:     alert.Content,
		Priority:    alert.Level,
		StationId:   alert.StationId,
		StationName: alert.StationName,
		DeviceId:    alert.DeviceId,
		Sn:          alert.Sn,
		AlertId:     alert.AlertId,
		CreatedBy:   nullInt64(uid),
	}
	created, err := createTicketCore(l.ctx, l.svcCtx, tid, tk)
	if err != nil {
		return nil, err
	}
	resp := &pb.TransferAlertToTicketResp{TicketId: created.TicketId, TicketNo: created.TicketNo}
	// 自动派单（可选；失败不回滚建单，返回单据由人工派）
	if in.AutoDispatch {
		auid, aname, _, derr := dispatchTicketCore(l.ctx, l.svcCtx, tid, created.TicketId, 0, acceptTimeoutMin(l.svcCtx))
		if derr == nil {
			resp.AssigneeUid = auid
			resp.AssigneeName = aname
		}
	}
	return resp, nil
}
