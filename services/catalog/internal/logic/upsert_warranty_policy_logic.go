package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpsertWarrantyPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertWarrantyPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertWarrantyPolicyLogic {
	return &UpsertWarrantyPolicyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpsertWarrantyPolicy 按 SKU 配置质保策略（period_months + start_rule，合同域执行时快照）。
func (l *UpsertWarrantyPolicyLogic) UpsertWarrantyPolicy(in *pb.UpsertWarrantyPolicyReq) (*pb.UpsertWarrantyPolicyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.SkuId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("sku_id 必填")
	}
	if in.PeriodMonths <= 0 {
		return nil, errPolicyPeriodBad
	}
	switch in.StartRule {
	case "ACTIVATION", "RECEIPT":
	default:
		return nil, errStartRuleBad
	}
	if _, err := l.svcCtx.Models.Sku.FindOne(l.ctx, tid, in.SkuId); err != nil {
		if err == model.ErrNotFound {
			return nil, errSkuNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}

	op := opUID(l.ctx)
	err = l.svcCtx.Models.WarrantyPolicy.Upsert(l.ctx, &model.WarrantyPolicy{
		PolicyId:     l.svcCtx.Snowflake.MustNextID(),
		SkuId:        in.SkuId,
		PeriodMonths: int64(in.PeriodMonths),
		StartRule:    in.StartRule,
		TenantId:     tid,
		CreatedBy:    toNullInt64(op),
		UpdatedBy:    toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpsertWarrantyPolicyResp{}, nil
}
