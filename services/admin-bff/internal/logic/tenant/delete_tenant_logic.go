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

type DeleteTenantLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTenantLogic {
	return &DeleteTenantLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteTenant 软删（identity 校验租户下无用户）。
func (l *DeleteTenantLogic) DeleteTenant(req *types.IDPath) (*types.SimpleResp, error) {
	tid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 非法")
	}
	if _, err := l.svcCtx.Identity.DeleteTenant(l.ctx, &pb.DeleteTenantReq{TenantId: tid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.ID}, nil
}
