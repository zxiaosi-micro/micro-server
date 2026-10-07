package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ImpersonateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewImpersonateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImpersonateLogic {
	return &ImpersonateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Impersonate 模拟他人（L2，02 §13 控制类审计）：
// 操作人会话须 step-up（EffectiveLevel≥2），否则 10406 提级；
// 为目标用户建 ADMIN_WEB 会话并签发；auth.impersonate 审计留 imp_by。
// 注：jwtauth.Claims 无 imp_by 字段——以审计事件 + 会话列表留痕，S7 演进可扩 claims。
func (l *ImpersonateLogic) Impersonate(in *pb.ImpersonateReq) (*pb.ImpersonateResp, error) {
	impBy := opUID(l.ctx)
	impSid := ctxkit.SID(l.ctx)
	if impBy == 0 || impSid == "" {
		return nil, errcode.ErrTokenInvalid.WithMsg("缺少操作人上下文")
	}

	// ① 操作人须 step-up（L2）
	level, err := l.svcCtx.Sessions.EffectiveLevel(l.ctx, impSid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if level < 2 {
		return nil, errcode.ErrStepUpRequired
	}

	// ② 目标用户校验
	if in.TargetUid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("target_uid 必填")
	}
	target, err := l.svcCtx.Models.User.FindOneByUID(l.ctx, in.TargetUid)
	if err != nil {
		return nil, errUserNotFound
	}
	if target.Status != 1 {
		return nil, errcode.ErrAccountDisabled
	}

	// ③ 目标会话（操作端视角）+ auth_cache
	client := ctxkit.Client(l.ctx)
	if client == "" {
		client = ctxkit.ClientAdminWeb
	}
	if err := ensureAuthCache(l.ctx, l.svcCtx, target.TenantId, target.UserId); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	access, refresh, accessExp, refreshExp, _, err := issueTokens(l.ctx, l.svcCtx, target.UserId, client)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	// ④ 审计（100% 落 outbox：模拟他人属最高敏操作）
	auditLogin(l.ctx, l.svcCtx.Conn, target.UserId, target.TenantId, "auth.impersonate")
	l.Infof("[审计] impersonate imp_by=%d -> target=%d sid 会话建立", impBy, target.UserId)

	return &pb.ImpersonateResp{
		Tokens: tokenPair(access, refresh, accessExp, refreshExp),
		ImpBy:  impBy,
	}, nil
}
