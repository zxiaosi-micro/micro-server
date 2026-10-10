package logic

import (
	"micro-server/services/station/pb"
)

func (l *PingLogic) Ping(_ *pb.PingReq) (*pb.PingResp, error) {
	return &pb.PingResp{Pong: "pong"}, nil
}
