package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStaffLogic {
	return &ListStaffLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListStaffLogic) ListStaff(in *pb.ListStaffReq) (*pb.ListStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Staff.FindPage(l.ctx, tid, in.PartyId, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListStaffResp{Total: total}
	for _, s := range list {
		resp.List = append(resp.List, &pb.StaffItem{
			StaffId:    s.StaffId,
			PartyId:    s.PartyId,
			UserId:     s.UserId.Int64,
			Name:       s.Name,
			StaffType:  s.StaffType,
			SkillTags:  s.SkillTags.String,
			WorkRegion: s.WorkRegion.String,
			Status:     int32(s.Status),
			CreatedAt:  s.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
