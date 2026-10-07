package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateOrgLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateOrgLogic {
	return &UpdateOrgLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateOrg 编辑组织（父节点不可指向自身/后代——环检测在树遍历层，S4 组织域补全）。
func (l *UpdateOrgLogic) UpdateOrg(in *pb.UpdateOrgReq) (*pb.UpdateOrgResp, error) {
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
	if in.ParentId == in.OrgId {
		return nil, errcode.ErrBadRequest.WithMsg("父节点不可指向自身")
	}
	if err := l.svcCtx.Models.Org.Update(l.ctx, tid, in.OrgId, in.ParentId, in.Name, int64(in.Sort), opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateOrgResp{}, nil
}
