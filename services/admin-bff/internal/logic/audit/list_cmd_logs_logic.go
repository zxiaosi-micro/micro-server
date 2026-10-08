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

type ListCmdLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 指令审计(perm: audit:cmd:list)
func NewListCmdLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCmdLogsLogic {
	return &ListCmdLogsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListCmdLogsLogic) ListCmdLogs(req *types.CmdAuditListReq) (resp *types.CmdAuditListResp, err error) {
	r, err := l.svcCtx.Audit.ListCmdLog(l.ctx, &auditPb.ListCmdLogReq{
		Sn: req.SN, CmdId: req.CmdID, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.CmdAuditListResp{Total: r.Total}
	for _, a := range r.List {
		resp.List = append(resp.List, types.CmdAuditItem{
			CmdAuditID: strconv.FormatInt(a.CmdAuditId, 10), CmdID: a.CmdId, SN: a.Sn,
			UID: strconv.FormatInt(a.Uid, 10), Action: a.Action, PayloadJson: a.PayloadJson,
			Result: a.Result, Error: a.Error, TraceID: a.TraceId, CreatedAt: a.CreatedAt,
		})
	}
	return resp, nil
}
