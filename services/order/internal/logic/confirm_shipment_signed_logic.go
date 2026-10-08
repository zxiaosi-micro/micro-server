package logic

import (
	"context"
	"time"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ConfirmShipmentSignedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmShipmentSignedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmShipmentSignedLogic {
	return &ConfirmShipmentSignedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConfirmShipmentSigned 签收确认：shipment_signed 事件驱动质保起算（FR-ORD-005/FR-CTR-005，按策略）。
func (l *ConfirmShipmentSignedLogic) ConfirmShipmentSigned(in *pb.ConfirmShipmentSignedReq) (*pb.ConfirmShipmentSignedResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	sc := l.svcCtx

	sh, err := sc.Models.Shipment.FindOneByNo(l.ctx, tid, in.ShipmentNo)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errShipmentBad
		}
		return nil, err
	}

	// 签收设备 SN 清单（质保起算对象；幂等——重复签收 CAS 拦截）
	items, err := sc.Models.ShipmentItem.ListByShipment(l.ctx, tid, sh.ShipmentId)
	if err != nil {
		return nil, err
	}
	var sns []string
	for _, it := range items {
		if s := nullStr(it.Sn); s != "" {
			sns = append(sns, s)
		}
	}

	now := time.Now()
	uid := ctxkit.UID(l.ctx)
	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Shipment.CASStatusTx(ctx, session, tid, sh.ShipmentId,
			[]string{"PENDING", "IN_TRANSIT"}, "SIGNED"); err != nil {
			return err
		}
		if err := sc.Models.Shipment.MarkSignedTx(ctx, session, tid, sh.ShipmentId, in.SignedBy, now); err != nil {
			return err
		}
		return eventbus.Emit(ctx, session, shipmentSignedEvent(tid, sh.OrderNo, sh.ShipmentNo, sns, in.SignedBy, now.UnixMilli()))
	})
	if err != nil {
		if err == model.ErrStatusConflict {
			return nil, errShipmentStatus.WithMsg("已签收(重复确认)")
		}
		return nil, err
	}

	l.Infof("发货单已签收 shipment_no=%s order_no=%s sns=%v by=%d", sh.ShipmentNo, sh.OrderNo, sns, uid)
	return &pb.ConfirmShipmentSignedResp{}, nil
}
