package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSagaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSagaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSagaLogic {
	return &GetSagaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetSaga Saga 进度详情（前端进度条可视化；步骤状态按 current_step 推导）。
func (l *GetSagaLogic) GetSaga(in *pb.GetSagaReq) (*pb.GetSagaResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.SagaId == 0 && in.OrderNo == "" {
		return nil, errSagaNotFound
	}
	var saga *model.Saga
	if in.SagaId > 0 {
		saga, err = l.svcCtx.Models.Saga.FindOneScoped(l.ctx, tid, in.SagaId)
	} else {
		saga, err = l.svcCtx.Models.Saga.FindOneByOrderNo(l.ctx, tid, in.OrderNo)
	}
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errSagaNotFound
		}
		return nil, err
	}
	return &pb.GetSagaResp{Saga: buildSagaDetail(saga)}, nil
}

// buildSagaDetail 步骤状态推导：
//   - 已过步骤 → DONE；当前步骤按整体状态映射；未达步骤 → PENDING；
//   - 非场站单的步骤6 → SKIPPED；整体 MANUAL → 当前步骤 MANUAL。
func buildSagaDetail(saga *model.Saga) *pb.SagaDetail {
	d := &pb.SagaDetail{
		SagaId:      saga.SagaId,
		OrderNo:     saga.OrderNo,
		OrderType:   saga.OrderType,
		CurrentStep: int32(saga.CurrentStep),
		Status:      saga.Status,
		RetryCount:  int32(saga.RetryCount),
		LastError:   saga.LastError,
		UpdatedAt:   saga.UpdatedAt.UnixMilli(),
	}
	if saga.NextRetryAt.Valid {
		d.NextRetryAt = saga.NextRetryAt.Time.UnixMilli()
	}
	for s := int64(stepCreate); s <= stepStation; s++ {
		st := &pb.SagaStep{Step: int32(s), Name: stepName[s], IdemKey: stepIdemKey[s]}
		switch {
		case s == stepStation && saga.OrderType != "STATION":
			st.Status = "SKIPPED"
		case s < saga.CurrentStep || saga.Status == "DONE":
			st.Status = "DONE"
		case s == saga.CurrentStep:
			switch {
			case saga.Status == "MANUAL":
				st.Status = "MANUAL"
			case saga.Status == "CANCELLED":
				st.Status = "CANCELLED"
			case s == stepPay && saga.Status == "RUNNING":
				// 等待支付：RUNNING（对用户即"进行中"）
				st.Status = "RUNNING"
			default:
				st.Status = "RUNNING"
			}
		default:
			st.Status = "PENDING"
		}
		d.Steps = append(d.Steps, st)
	}
	return d
}
