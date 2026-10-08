package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProductLogic {
	return &CreateProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateProductLogic) CreateProduct(in *pb.CreateProductReq) (*pb.CreateProductResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("name 必填")
	}

	productId := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Product.Insert(l.ctx, &model.Product{
		ProductId: productId,
		Name:      in.Name,
		Category:  toNullString(in.Category),
		Status:    1,
		Remark:    toNullString(in.Remark),
		TenantId:  tid,
		CreatedBy: toNullInt64(op),
		UpdatedBy: toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateProductResp{ProductId: productId}, nil
}
