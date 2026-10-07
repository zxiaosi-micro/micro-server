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

type DeleteOrgLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteOrgLogic {
	return &DeleteOrgLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteOrgLogic) DeleteOrg(req *types.IDPath) (*types.SimpleResp, error) {
	oid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("org_id 非法")
	}
	if _, err := l.svcCtx.Identity.DeleteOrg(l.ctx, &pb.DeleteOrgReq{OrgId: oid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.ID}, nil
}
