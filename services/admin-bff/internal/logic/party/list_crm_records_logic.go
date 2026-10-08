// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCrmRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 跟进记录(perm: party:party:list)
func NewListCrmRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCrmRecordsLogic {
	return &ListCrmRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListCrmRecordsLogic) ListCrmRecords(req *types.IDPath) (resp *types.CrmRecordListResp, err error) {
	r, err := l.svcCtx.Party.ListCrmRecord(l.ctx, &partyPb.ListCrmRecordReq{PartyId: parseID(req.ID)})
	if err != nil {
		return nil, err
	}
	resp = &types.CrmRecordListResp{}
	for _, c := range r.List {
		resp.List = append(resp.List, types.CrmRecordItem{
			RecordID: strconv.FormatInt(c.RecordId, 10), PartyID: strconv.FormatInt(c.PartyId, 10),
			Content: c.Content, NextFollowAt: c.NextFollowAt,
			CreatedBy: strconv.FormatInt(c.CreatedBy, 10), CreatedAt: c.CreatedAt,
		})
	}
	return resp, nil
}
