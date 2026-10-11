package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
)

type CreateTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTicketLogic {
	return &CreateTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 工单（FR-OPS-004/005/008）----
func (l *CreateTicketLogic) CreateTicket(in *pb.CreateTicketReq) (*pb.CreateTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	row := &model.Ticket{
		TicketId:       l.svcCtx.Snowflake.MustNextID(),
		Type:           in.Type,
		Title:          in.Title,
		Content:        in.Content,
		Priority:       in.Priority,
		StationId:      in.StationId,
		DeviceId:       in.DeviceId,
		Sn:             in.Sn,
		AlertId:        in.AlertId,
		CreatedBy:      nullInt64(ctxkit.UID(l.ctx)),
		ClientActionId: nullString(in.ClientActionId),
		PartyId:        nullInt64(in.PartyId),
	}
	created, err := createTicketCore(l.ctx, l.svcCtx, tid, row)
	if err != nil {
		return nil, err
	}
	return &pb.CreateTicketResp{TicketId: created.TicketId, TicketNo: created.TicketNo}, nil
}
