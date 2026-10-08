package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateSKULogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSKULogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSKULogic {
	return &CreateSKULogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateSKU 新建 SKU（type=STANDARD/EXT_WARRANTY——延保也是 SKU，下单链路统一）。
func (l *CreateSKULogic) CreateSKU(in *pb.CreateSKUReq) (*pb.CreateSKUResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.ProductId <= 0 || in.Code == "" || in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("product_id/code/name 必填")
	}
	if in.Type == "" {
		in.Type = "STANDARD"
	}
	switch in.Type {
	case "STANDARD", "EXT_WARRANTY":
	default:
		return nil, errSkuTypeBad
	}
	if _, err := l.svcCtx.Models.Product.FindOne(l.ctx, tid, in.ProductId); err != nil {
		if err == model.ErrNotFound {
			return nil, errProductNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}

	skuId := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Sku.Insert(l.ctx, &model.Sku{
		SkuId:     skuId,
		ProductId: in.ProductId,
		Code:      in.Code,
		Name:      in.Name,
		Type:      in.Type,
		Spec:      toNullString(in.Spec),
		Status:    1,
		TenantId:  tid,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		if isDupKey(err) && dupKeyName(err) == "uk_sku_tenant_code" {
			return nil, errSkuCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateSKUResp{SkuId: skuId}, nil
}
