// confcenter 运行参数（logic 层访问点）。
package logic

import (
	"micro-server/services/device/internal/confcenter"
)

func confcenterCurrent() confcenter.DeviceConf { return confcenter.Current() }
