package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ResolveReconcileTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveReconcileTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveReconcileTaskLogic {
	return &ResolveReconcileTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveReconcileTaskLogic) ResolveReconcileTask(in *pb.ResolveReconcileTaskReq) (*pb.ResolveReconcileTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := ResolveReconcileTaskInternal(l.ctx, l.svcCtx, tid, in.TaskId, in.Resolution, in.Remark, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.ResolveReconcileTaskResp{}, nil
}
