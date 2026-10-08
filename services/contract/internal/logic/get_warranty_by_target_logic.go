package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWarrantyByTargetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWarrantyByTargetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWarrantyByTargetLogic {
	return &GetWarrantyByTargetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWarrantyByTargetLogic) GetWarrantyByTarget(in *pb.GetWarrantyByTargetReq) (*pb.GetWarrantyByTargetResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.TargetType != "DEVICE" && in.TargetType != "STATION" {
		return nil, errTargetBad
	}
	list, err := l.svcCtx.Models.Warranty.FindByTarget(l.ctx, tid, in.TargetType, in.TargetId, in.Sn)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetWarrantyByTargetResp{}
	for _, w := range list {
		resp.List = append(resp.List, buildWarrantyItem(w))
	}
	return resp, nil
}
