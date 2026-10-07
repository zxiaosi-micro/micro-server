package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateTenantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTenantLogic {
	return &UpdateTenantLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateTenant 编辑租户（name/plan/status；停用租户的账号处置由 S7 运维域编排）。
func (l *UpdateTenantLogic) UpdateTenant(in *pb.UpdateTenantReq) (*pb.UpdateTenantResp, error) {
	if in.TenantId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 必填")
	}
	cur, err := l.svcCtx.Models.Tenant.FindOne(l.ctx, in.TenantId)
	if err != nil {
		return nil, errTenantNotFound
	}
	name, plan := cur.Name, cur.Plan
	if in.Name != "" {
		name = in.Name
	}
	if in.Plan != "" {
		plan = in.Plan
	}
	status := cur.Status
	if in.Status > 0 {
		status = int64(in.Status)
	}
	if err := l.svcCtx.Models.Tenant.Update(l.ctx, in.TenantId, name, plan, status, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateTenantResp{}, nil
}
