// create_ticket_logic.go · 报修（AFTER_SALE 工单；requireParty 校验设备归属，
// party_id 落 ops.ticket 供隔离查询；photos 并入 content 留痕）。

package customer

import (
	"context"
	"encoding/json"
	"strings"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateTicketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTicketLogic {
	return &CreateTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateTicketLogic) CreateTicket(req *types.CreateTicketReq) (resp *types.CreateTicketResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) == "" || (req.DeviceId == "" && req.SN == "") {
		return nil, errcode.ErrBadRequest.WithMsg("title 与 device_id/sn 必填其一")
	}

	var deviceId int64
	sn := req.SN
	if req.DeviceId != "" {
		dev, err := ownDeviceByID(l.ctx, l.svcCtx, parseI64(req.DeviceId), partyId)
		if err != nil {
			return nil, err
		}
		deviceId = dev.DeviceId
		sn = dev.Sn
	} else if _, err := ownDeviceBySN(l.ctx, l.svcCtx, sn, partyId); err != nil {
		return nil, err
	}

	content := req.Content
	if len(req.Photos) > 0 {
		photos, _ := json.Marshal(req.Photos)
		content = strings.TrimSpace(content) + "\nphotos=" + string(photos)
	}

	out, err := l.svcCtx.Ops.CreateTicket(l.ctx, &opsb.CreateTicketReq{
		Type:           "AFTER_SALE",
		Title:          req.Title,
		Content:        content,
		DeviceId:       deviceId,
		Sn:             sn,
		ClientActionId: req.ClientActionId,
		PartyId:        partyId,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateTicketResp{TicketId: i64s(out.TicketId), TicketNo: out.TicketNo}, nil
}
