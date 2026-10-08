package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type RetrySagaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetrySagaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetrySagaLogic {
	return &RetrySagaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RetrySaga 人工介入后重推（前端人工介入按钮）：MANUAL/RUNNING → RUNNING + 立即到期。
func (l *RetrySagaLogic) RetrySaga(in *pb.RetrySagaReq) (*pb.RetrySagaResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.SagaId == 0 {
		return nil, errSagaNotFound
	}
	sc := l.svcCtx
	saga, err := sc.Models.Saga.FindOneScoped(l.ctx, tid, in.SagaId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errSagaNotFound
		}
		return nil, err
	}
	if saga.Status != "MANUAL" && saga.Status != "RUNNING" {
		return nil, errSagaNotRetryable.WithMsg("当前状态: " + saga.Status)
	}

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return sc.Models.Saga.ResetForRetryTx(ctx, session, tid, in.SagaId)
	})
	if err != nil {
		if err == model.ErrStatusConflict {
			return nil, errSagaNotRetryable
		}
		return nil, err
	}

	l.Infof("saga 人工重推 saga_id=%d order_no=%s by=%d remark=%s",
		in.SagaId, saga.OrderNo, ctxkit.UID(l.ctx), in.Remark)

	// 立即推进（异步加速器；ResetForRetryTx 已置 next_retry_at=now，失败由 cron 兜底）
	KickAdvance(sc, tid, saga.OrderNo)
	return &pb.RetrySagaResp{}, nil
}
