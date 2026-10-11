package analytics

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	anpb "micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBIWhitelistLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// BI 白名单元数据(查询构建器数据源)
func NewListBIWhitelistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBIWhitelistLogic {
	return &ListBIWhitelistLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListBIWhitelistLogic) ListBIWhitelist() (resp *types.BIWhitelistResp, err error) {
	out, err := l.svcCtx.Analytics.ListBIWhitelist(l.ctx, &anpb.ListBIWhitelistReq{})
	if err != nil {
		return nil, err
	}
	resp = &types.BIWhitelistResp{Tables: []types.BIWhitelistTable{}}
	for _, t := range out.Tables {
		tw := types.BIWhitelistTable{Table: t.Table, Comment: t.Comment, Columns: []types.BIWhitelistColumn{}}
		for _, c := range t.Columns {
			tw.Columns = append(tw.Columns, types.BIWhitelistColumn{
				Column: c.Column, Kind: c.Kind, Aggs: c.Aggs, Ops: c.Ops, Comment: c.Comment,
			})
		}
		resp.Tables = append(resp.Tables, tw)
	}
	return resp, nil
}
