package logic

import (
	"context"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type AddCrmRecordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddCrmRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddCrmRecordLogic {
	return &AddCrmRecordLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// AddCrmRecord 追加跟进记录（追加式，不提供改删）。
func (l *AddCrmRecordLogic) AddCrmRecord(in *pb.AddCrmRecordReq) (*pb.AddCrmRecordResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.PartyId <= 0 || in.Content == "" {
		return nil, errcode.ErrBadRequest.WithMsg("party_id/content 必填")
	}
	rid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.CrmRecord.Insert(l.ctx, &model.CrmRecord{
		RecordId:     rid,
		PartyId:      in.PartyId,
		Content:      in.Content,
		NextFollowAt: toNullTime(in.NextFollowAt),
		TenantId:     tid,
		CreatedBy:    toNullInt64(op),
		UpdatedBy:    toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.AddCrmRecordResp{RecordId: rid}, nil
}
