// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveReconcileTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResolveReconcileTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveReconcileTaskLogic {
	return &ResolveReconcileTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResolveReconcileTaskLogic) ResolveReconcileTask(req *types.ReconcileResolveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.ResolveReconcileTask(l.ctx, &finpb.ResolveReconcileTaskReq{
		TaskId: req.Id, Resolution: req.Resolution, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
