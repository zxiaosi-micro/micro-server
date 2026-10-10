// station logic 结构体定义（方法体在 kernel 文件；goctl 重新生成时删除对应 _logic.go 骨架保留本文件）。
package logic

import (
	"context"

	"micro-server/services/station/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStationLogic {
	return &CreateStationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetStationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationLogic {
	return &GetStationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListStationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationLogic {
	return &ListStationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type BindDevicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindDevicesLogic {
	return &BindDevicesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListStationDevicesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStationDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationDevicesLogic {
	return &ListStationDevicesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type SaveTopologyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveTopologyLogic {
	return &SaveTopologyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetTopologyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTopologyLogic {
	return &GetTopologyLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetStationMonitorLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStationMonitorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationMonitorLogic {
	return &GetStationMonitorLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type AddStationStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddStationStaffLogic {
	return &AddStationStaffLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type RemoveStationStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRemoveStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveStationStaffLogic {
	return &RemoveStationStaffLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type ListStationStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationStaffLogic {
	return &ListStationStaffLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type GetDeviceStationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDeviceStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceStationLogic {
	return &GetDeviceStationLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

type PingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PingLogic {
	return &PingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}
