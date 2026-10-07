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

type UpdateTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTenantLogic {
	return &UpdateTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateTenant 编辑租户（name/plan/status）。
func (l *UpdateTenantLogic) UpdateTenant(req *types.TenantUpdateReq) (*types.SimpleResp, error) {
	tid, err := strconv.ParseInt(req.TenantID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateTenant(l.ctx, &pb.UpdateTenantReq{
		TenantId: tid, Name: req.Name, Plan: req.Plan, Status: int32(req.Status),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.TenantID}, nil
}
