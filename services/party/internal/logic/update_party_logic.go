package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdatePartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdatePartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePartyLogic {
	return &UpdatePartyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdatePartyLogic) UpdateParty(in *pb.UpdatePartyReq) (*pb.UpdatePartyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" || len(in.Type) == 0 {
		return nil, errcode.ErrBadRequest.WithMsg("name/type 必填")
	}
	for _, t := range in.Type {
		if !validPartyType(t) {
			return nil, errPartyTypeBad
		}
	}
	status := int64(in.Status)
	if status == 0 {
		status = 1 // 0 = 不修改（编辑页无状态字段时保持正常态）
	}
	if _, err := l.svcCtx.Models.Party.FindOne(l.ctx, tid, in.PartyId); err != nil {
		return nil, partyErr(err)
	}
	err = l.svcCtx.Models.Party.UpdateColumns(l.ctx, tid, in.PartyId, in.Name, marshalJSON(in.Type), status,
		toNullString(in.CreditCode), toNullString(in.Region), toNullString(in.Address), toNullString(in.Remark),
		opUID(l.ctx))
	if err != nil {
		if isDupKey(err) && dupKeyName(err) == "uk_party_tenant_credit" {
			return nil, errCreditCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdatePartyResp{}, nil
}
