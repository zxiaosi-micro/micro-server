package tenant

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTenantLogic {
	return &CreateTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateTenant 建租户（tenant_code UK 冲突 → identity 1101012）。
func (l *CreateTenantLogic) CreateTenant(req *types.TenantCreateReq) (*types.SimpleResp, error) {
	resp, err := l.svcCtx.Identity.CreateTenant(l.ctx, &pb.CreateTenantReq{
		TenantCode: req.TenantCode, Name: req.Name, Plan: req.Plan, Quota: req.Quota,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(resp.TenantId, 10)}, nil
}
