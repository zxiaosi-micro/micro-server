package tenant

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateTenantQuotaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateTenantQuotaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTenantQuotaLogic {
	return &UpdateTenantQuotaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateTenantQuota 配额 JSON 全量替换（tenantx 配额源；JSON 合法性 identity 强校验）。
func (l *UpdateTenantQuotaLogic) UpdateTenantQuota(req *types.TenantQuotaReq) (*types.SimpleResp, error) {
	tid, err := strconv.ParseInt(req.TenantID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateTenantQuota(l.ctx, &pb.UpdateTenantQuotaReq{
		TenantId: tid, Quota: req.Quota,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.TenantID}, nil
}
