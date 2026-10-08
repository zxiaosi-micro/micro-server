package logic

import (
	"context"

	"micro-server/services/party/internal/model"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type CreateStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStaffLogic {
	return &CreateStaffLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateStaff 新建服务商员工（user_id 逻辑引用 identity.user；skill_tags 为 S7 派单匹配依据）。
func (l *CreateStaffLogic) CreateStaff(in *pb.CreateStaffReq) (*pb.CreateStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.PartyId <= 0 || in.Name == "" {
		return nil, errcode.ErrBadRequest.WithMsg("party_id/name 必填")
	}
	if in.StaffType == "" {
		in.StaffType = "ENGINEER"
	}
	switch in.StaffType {
	case "ENGINEER", "SALES", "OPS", "ADMIN":
	default:
		return nil, errStaffTypeBad
	}
	if _, err := l.svcCtx.Models.Party.FindOne(l.ctx, tid, in.PartyId); err != nil {
		return nil, partyErr(err)
	}

	sid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	_, err = l.svcCtx.Models.Staff.Insert(l.ctx, &model.Staff{
		StaffId:    sid,
		PartyId:    in.PartyId,
		UserId:     toNullInt64(in.UserId),
		Name:       in.Name,
		StaffType:  in.StaffType,
		SkillTags:  toNullString(marshalJSON(in.SkillTags)),
		WorkRegion: toNullString(in.WorkRegion),
		Status:     1,
		TenantId:   tid,
		CreatedBy:  toNullInt64(op),
		UpdatedBy:  toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.CreateStaffResp{StaffId: sid}, nil
}
