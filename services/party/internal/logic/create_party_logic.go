package logic

import (
	"context"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreatePartyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePartyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePartyLogic {
	return &CreatePartyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateParty 新建参与方（type JSON SET 语义：多类型可并存）。
func (l *CreatePartyLogic) CreateParty(in *pb.CreatePartyReq) (*pb.CreatePartyResp, error) {
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

	pid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Party.Insert(l.ctx, &model.Party{
		PartyId:    pid,
		Name:       in.Name,
		Type:       marshalJSON(in.Type),
		Status:     1,
		CreditCode: toNullString(in.CreditCode),
		Region:     toNullString(in.Region),
		Address:    toNullString(in.Address),
		Remark:     toNullString(in.Remark),
		TenantId:   tid,
		CreatedBy:  toNullInt64(op),
		UpdatedBy:  toNullInt64(op),
	})
	if err != nil {
		if isDupKey(err) && dupKeyName(err) == "uk_party_tenant_credit" {
			return nil, errCreditCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreatePartyResp{PartyId: pid}, nil
}

// validPartyType 参与方类型值域（01 §5.2）。
func validPartyType(t string) bool {
	switch t {
	case "CUSTOMER", "DEALER", "SUPPLIER", "ENTERPRISE", "RECYCLER":
		return true
	}
	return false
}
