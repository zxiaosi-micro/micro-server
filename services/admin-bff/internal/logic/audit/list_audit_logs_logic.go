// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package audit

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	auditPb "micro-server/services/audit/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAuditLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 操作审计(perm: audit:log:list)
func NewListAuditLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAuditLogsLogic {
	return &ListAuditLogsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAuditLogsLogic) ListAuditLogs(req *types.AuditLogListReq) (resp *types.AuditLogListResp, err error) {
	r, err := l.svcCtx.Audit.ListLog(l.ctx, &auditPb.ListLogReq{
		Action: req.Action, Uid: parseID(req.UID), TargetType: req.TargetType, TargetId: req.TargetID,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AuditLogListResp{Total: r.Total}
	for _, a := range r.List {
		resp.List = append(resp.List, types.AuditLogItem{
			LogID: strconv.FormatInt(a.LogId, 10), TraceID: a.TraceId, UID: strconv.FormatInt(a.Uid, 10),
			Action: a.Action, TargetType: a.TargetType, TargetID: a.TargetId,
			BeforeJson: a.BeforeJson, AfterJson: a.AfterJson, Result: a.Result,
			Client: a.Client, OnBehalfOf: strconv.FormatInt(a.OnBehalfOf, 10), CreatedAt: a.CreatedAt,
		})
	}
	return resp, nil
}
