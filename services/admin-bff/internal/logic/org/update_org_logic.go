package org

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateOrgLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateOrgLogic {
	return &UpdateOrgLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateOrgLogic) UpdateOrg(req *types.OrgUpdateReq) (*types.SimpleResp, error) {
	oid, err := strconv.ParseInt(req.OrgID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("org_id 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateOrg(l.ctx, &pb.UpdateOrgReq{
		OrgId: oid, ParentId: parseID(req.ParentID), Name: req.Name, Sort: int32(req.Sort),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.OrgID}, nil
}
