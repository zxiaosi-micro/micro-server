package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
)

type CreateInspectionTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInspectionTemplateLogic {
	return &CreateInspectionTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateInspectionTemplateLogic) CreateInspectionTemplate(in *pb.CreateInspectionTemplateReq) (*pb.CreateInspectionTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	row := &model.InspectionTemplate{
		TemplateId:  l.svcCtx.Snowflake.MustNextID(),
		Name:        in.Name,
		ItemsSchema: in.ItemsSchema,
		TenantId:    tid,
		CreatedBy:   nullInt64(ctxkit.UID(l.ctx)),
	}
	if err := l.svcCtx.Models.InsTmpl.InsertTx(l.ctx, l.svcCtx.Conn, row); err != nil {
		return nil, err
	}
	return &pb.CreateInspectionTemplateResp{TemplateId: row.TemplateId}, nil
}
