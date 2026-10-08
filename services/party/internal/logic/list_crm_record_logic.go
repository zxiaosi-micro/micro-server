package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListCrmRecordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCrmRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCrmRecordLogic {
	return &ListCrmRecordLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListCrmRecordLogic) ListCrmRecord(in *pb.ListCrmRecordReq) (*pb.ListCrmRecordResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.CrmRecord.ListByParty(l.ctx, tid, in.PartyId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListCrmRecordResp{}
	for _, r := range list {
		resp.List = append(resp.List, &pb.CrmRecordItem{
			RecordId:     r.RecordId,
			PartyId:      r.PartyId,
			Content:      r.Content,
			NextFollowAt: nullTimeMilli(r.NextFollowAt),
			CreatedBy:    r.CreatedBy.Int64,
			CreatedAt:    r.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
