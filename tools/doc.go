// Package tools 是 micro-server 工具箱占位（S0-01）。
//
// 各工具自 S2-05 起按子目录逐个落地（每个工具一个 package main 子目录）：
// migrate（建库+成对迁移）/ seed（种子数据）/ envcheck（中间件连通 5 项）/
// dodcheck（上线 DoD 静态把关）/ apitypes（.api → TS 类型）/ mqinit（topic 预创建）/
// devicesim（设备模拟器）/ reconcile（对账）/ newsvc（新服务脚手架）/
// keygen（密钥三件套）/ configpush（配置正本推送 etcd）。
package tools
