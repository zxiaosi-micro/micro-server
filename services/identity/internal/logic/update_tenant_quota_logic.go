package logic

import (
	"context"
	"encoding/json"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateTenantQuotaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTenantQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTenantQuotaLogic {
	return &UpdateTenantQuotaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateTenantQuota 配额 JSON 全量替换（tenantx 配额源；JSON 合法性先校验）。
func (l *UpdateTenantQuotaLogic) UpdateTenantQuota(in *pb.UpdateTenantQuotaReq) (*pb.UpdateTenantQuotaResp, error) {
	if in.TenantId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 必填")
	}
	if in.Quota != "" {
		var v map[string]any
		if err := json.Unmarshal([]byte(in.Quota), &v); err != nil {
			return nil, errcode.ErrBadRequest.WithMsg("quota 必须是合法 JSON")
		}
	}
	if _, err := l.svcCtx.Models.Tenant.FindOne(l.ctx, in.TenantId); err != nil {
		return nil, errTenantNotFound
	}
	if err := l.svcCtx.Models.Tenant.UpdateQuota(l.ctx, in.TenantId, in.Quota, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateTenantQuotaResp{}, nil
}
