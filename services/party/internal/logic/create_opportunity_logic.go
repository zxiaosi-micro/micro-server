package logic

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateOpportunityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOpportunityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOpportunityLogic {
	return &CreateOpportunityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateOpportunity 新建商机（stage 初始 NEW；金额 DECIMAL 以字符串进出）。
func (l *CreateOpportunityLogic) CreateOpportunity(in *pb.CreateOpportunityReq) (*pb.CreateOpportunityResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.PartyId <= 0 || in.Title == "" {
		return nil, errcode.ErrBadRequest.WithMsg("party_id/title 必填")
	}
	amount, err := parseAmount(in.Amount)
	if err != nil {
		return nil, errAmountBad.WithCause(err)
	}

	oid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Opportunity.Insert(l.ctx, &model.Opportunity{
		OpportunityId:     oid,
		PartyId:           in.PartyId,
		Title:             in.Title,
		Stage:             "NEW",
		Amount:            amount,
		ExpectedCloseDate: toNullTime(in.ExpectedCloseDate),
		Remark:            toNullString(in.Remark),
		TenantId:          tid,
		CreatedBy:         toNullInt64(op),
		UpdatedBy:         toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateOpportunityResp{OpportunityId: oid}, nil
}

// parseAmount 金额字符串 → DECIMAL（两位小数四舍五入；空串 = NULL）。
func parseAmount(s string) (sql.NullFloat64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullFloat64{}, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return sql.NullFloat64{}, err
	}
	return sql.NullFloat64{Float64: math.Round(f*100) / 100, Valid: true}, nil
}
