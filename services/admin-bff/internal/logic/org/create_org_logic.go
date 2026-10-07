package org

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOrgLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateOrgLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrgLogic {
	return &CreateOrgLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateOrgLogic) CreateOrg(req *types.OrgCreateReq) (*types.SimpleResp, error) {
	resp, err := l.svcCtx.Identity.CreateOrg(l.ctx, &pb.CreateOrgReq{
		ParentId: parseID(req.ParentID), Name: req.Name, Sort: int32(req.Sort),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(resp.OrgId, 10)}, nil
}
