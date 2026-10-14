// logic 公共助手（confcenter 访问点）。
package logic

import (
	"micro-server/services/station/internal/confcenter"

	devclient "micro-server/services/device/device"
	devpb "micro-server/services/device/pb"
	"micro-server/services/station/internal/svc"
)

func confcenterCurrent() confcenter.StationConf { return confcenter.Current() }

// 别名（kernel 引用对齐 device 服务客户端形状）。
var (
	_                     = devpb.GetDeviceReq{}
	_ devclient.Device    = nil
	_ *svc.ServiceContext = nil
)
