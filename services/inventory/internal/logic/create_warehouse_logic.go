package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateWarehouseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWarehouseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWarehouseLogic {
	return &CreateWarehouseLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateWarehouseLogic) CreateWarehouse(in *pb.CreateWarehouseReq) (*pb.CreateWarehouseResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Code == "" || in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("code/name 必填")
	}

	wid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Warehouse.Insert(l.ctx, &model.Warehouse{
		WarehouseId: wid,
		Code:        in.Code,
		Name:        in.Name,
		Address:     toNullString(in.Address),
		Status:      1,
		TenantId:    tid,
		CreatedBy:   toNullInt64(op),
		UpdatedBy:   toNullInt64(op),
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errWarehouseCodeUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateWarehouseResp{WarehouseId: wid}, nil
}
