// device_ownership.go · requireParty 设备归属校验（水平越权拦截：设备不属于本人 party 即 403）。

package customer

import (
	"context"

	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"

	"github.com/zxiaosi-micro/micro-common/errcode"
)

// ownDeviceBySN 按 SN 取设备并校验归属（party_id 匹配；历史 NULL 归属行对客户不可见）。
func ownDeviceBySN(ctx context.Context, sc *svc.ServiceContext, sn string, partyId int64) (*dpb.DeviceView, error) {
	out, err := sc.Device.GetDeviceBySn(ctx, &dpb.GetDeviceBySnReq{Sn: sn})
	if err != nil {
		return nil, err
	}
	if out.GetDevice().GetPartyId() != partyId {
		return nil, errcode.ErrPermissionDenied
	}
	return out.Device, nil
}

// ownDeviceByID 按 device_id 取设备并校验归属。
func ownDeviceByID(ctx context.Context, sc *svc.ServiceContext, deviceID, partyId int64) (*dpb.DeviceView, error) {
	out, err := sc.Device.GetDevice(ctx, &dpb.GetDeviceReq{DeviceId: deviceID})
	if err != nil {
		return nil, err
	}
	if out.GetDevice().GetPartyId() != partyId {
		return nil, errcode.ErrPermissionDenied
	}
	return out.Device, nil
}
