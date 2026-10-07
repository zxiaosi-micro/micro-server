package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteOrgLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteOrgLogic {
	return &DeleteOrgLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteOrg 软删：有子组织或有挂靠用户时拒绝。
func (l *DeleteOrgLogic) DeleteOrg(in *pb.DeleteOrgReq) (*pb.DeleteOrgResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.OrgId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("org_id 必填")
	}
	if _, err := l.svcCtx.Models.Org.FindOne(l.ctx, tid, in.OrgId); err != nil {
		return nil, errOrgNotFound
	}
	children, err := l.svcCtx.Models.Org.CountChildren(l.ctx, tid, in.OrgId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if children > 0 {
		return nil, errHasChildren
	}
	users, err := l.svcCtx.Models.User.CountByOrg(l.ctx, tid, in.OrgId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if users > 0 {
		return nil, errOrgHasUsers
	}
	if err := l.svcCtx.Models.Org.SoftDelete(l.ctx, tid, in.OrgId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.DeleteOrgResp{}, nil
}
