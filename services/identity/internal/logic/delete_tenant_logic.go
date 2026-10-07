package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteTenantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteTenantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTenantLogic {
	return &DeleteTenantLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteTenant 软删：租户下仍有用户时拒绝（高危操作，02 §9.4）。
func (l *DeleteTenantLogic) DeleteTenant(in *pb.DeleteTenantReq) (*pb.DeleteTenantResp, error) {
	if in.TenantId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("tenant_id 必填")
	}
	if _, err := l.svcCtx.Models.Tenant.FindOne(l.ctx, in.TenantId); err != nil {
		return nil, errTenantNotFound
	}
	users, err := l.svcCtx.Models.User.CountByTenant(l.ctx, in.TenantId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if users > 0 {
		return nil, errTenantHasUsers
	}
	if err := l.svcCtx.Models.Tenant.SoftDelete(l.ctx, in.TenantId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.DeleteTenantResp{}, nil
}
