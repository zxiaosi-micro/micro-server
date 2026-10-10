package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"encoding/json"
	"github.com/zeromicro/go-zero/core/logx"
)

type MatchStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMatchStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MatchStaffLogic {
	return &MatchStaffLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// MatchStaff 派单候选（场站区域 + 技能标签匹配，FR-OPS-004/FR-PTY-005）。
func (l *MatchStaffLogic) MatchStaff(in *pb.MatchStaffReq) (*pb.MatchStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	var skills []string
	if in.RequiredSkills != "" {
		if err := json.Unmarshal([]byte(in.RequiredSkills), &skills); err != nil {
			return nil, errRuleBad
		}
	}
	list, err := matchStaffCore(l.ctx, l.svcCtx, tid, in.StationId, skills)
	if err != nil {
		return nil, err
	}
	resp := &pb.MatchStaffResp{}
	for _, s := range list {
		resp.List = append(resp.List, &pb.StaffCandidate{
			StaffId: s.StaffId, UserId: s.UserId, Name: s.Name,
			StaffType: s.StaffType, SkillTags: s.SkillTags, WorkRegion: s.WorkRegion,
		})
	}
	return resp, nil
}
