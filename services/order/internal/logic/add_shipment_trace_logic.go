package logic

import (
	"context"
	"time"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type AddShipmentTraceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddShipmentTraceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddShipmentTraceLogic {
	return &AddShipmentTraceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AddShipmentTrace 轨迹手工录入（FR-ORD-005 基线）；首条轨迹自动 PENDING → IN_TRANSIT。
func (l *AddShipmentTraceLogic) AddShipmentTrace(in *pb.AddShipmentTraceReq) (*pb.AddShipmentTraceResp, error) {
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
	if sh.Status == "SIGNED" {
		return nil, errShipmentStatus.WithMsg("已签收，不可追加轨迹")
	}

	traceTime := time.Now()
	if in.TraceTime > 0 {
		traceTime = time.UnixMilli(in.TraceTime)
	}
	uid := ctxkit.UID(l.ctx)

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		trace := &model.ShipmentTrace{
			TraceId: sc.Snowflake.MustNextID(), ShipmentId: sh.ShipmentId,
			Node: in.Node, TraceTime: traceTime,
			TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
		}
		if in.Description != "" {
			trace.Description = sqlString(in.Description)
		}
		if err := sc.Models.ShipmentTrace.InsertTx(ctx, session, trace); err != nil {
			return err
		}
		if sh.Status == "PENDING" {
			return sc.Models.Shipment.CASStatusTx(ctx, session, tid, sh.ShipmentId,
				[]string{"PENDING"}, "IN_TRANSIT")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.AddShipmentTraceResp{}, nil
}
