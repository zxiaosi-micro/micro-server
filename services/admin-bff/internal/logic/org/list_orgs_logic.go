package org

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListOrgsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListOrgsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrgsLogic {
	return &ListOrgsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListOrgs 组织平铺（前端组树）。
func (l *ListOrgsLogic) ListOrgs() (*types.OrgListResp, error) {
	resp, err := l.svcCtx.Identity.ListOrgs(l.ctx, &pb.ListOrgsReq{})
	if err != nil {
		return nil, err
	}
	list := make([]types.OrgItem, 0, len(resp.List))
	for _, o := range resp.List {
		list = append(list, types.OrgItem{
			OrgID:    strconv.FormatInt(o.OrgId, 10),
			ParentID: strconv.FormatInt(o.ParentId, 10),
			Name:     o.Name,
			Sort:     int(o.Sort),
		})
	}
	return &types.OrgListResp{List: list}, nil
}
