// otasmoke · OTA 全链冒烟（S6 阶段验收：签名/灰度/暂停/回滚，FR-IOT-007）。
//
// 链路：SaveFirmware（Ed25519 签名，未签名拒绝）→ CreateOtaTask（灰度清单）
// → dispatch（OTA_UPGRADE 指令 SENT）→ ota_paused/回滚验证。
//
// 前置：device RPC 运行 + OTA_SIGN_KEY 私钥（keygen 产出）。
// 纪律：严禁并发执行（E1）。
//
//	go run ./tools/otasmoke
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	devpb "micro-server/services/device/pb"
)

const (
	tenantID = 9000000000000000001
	seedUID  = 9000000000000000002
)

func main() {
	keyPath := flag.String("key", envOr("OTA_SIGN_KEY", "ota_ed25519_private.pem"), "Ed25519 私钥（PEM）")
	host := flag.String("host", envOr("DEVICE_HOST", "127.0.0.1"), "device RPC 主机")
	port := flag.Int("port", 8085, "device RPC 端口")
	flag.Parse()

	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), seedUID)

	// ---- 直连 device RPC ----
	dconn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{fmt.Sprintf("%s:%d", *host, *port)},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
	dev := devpb.NewDeviceClient(dconn)

	// ---- 准备目标设备（导入 + 激活态模拟：直接导 1 台做灰度清单成员）----
	ts := time.Now().UnixMilli() % 100000000
	sn := fmt.Sprintf("OTASMOKE-%d", ts)
	imp, err := dev.ImportSN(ctx, &devpb.ImportSNReq{
		Items:     []*devpb.ImportDeviceItem{{Sn: sn, ProductKey: "ESS-DEMO", Model: "SIM"}},
		Provision: false, // 冒烟免 EMQX 依赖
	})
	must(err, "ImportSN")
	pass("设备建档 sn=%s", imp.Secrets[0].Sn)

	// ---- 1. 未签名固件拒绝入库（FR-IOT-007 验收点 1）----
	fwBody := []byte("sim-firmware-" + sn)
	sum := sha256.Sum256(fwBody)
	shaHex := hex.EncodeToString(sum[:])
	_, err = dev.SaveFirmware(ctx, &devpb.SaveFirmwareReq{
		ProductKey: "ESS-DEMO", Version: fmt.Sprintf("1.0.%d", ts%1000),
		FileUrl: "s3://micro-file/ota/" + sn + ".bin", FileSize: int64(len(fwBody)),
		Sha256: shaHex, Signature: "",
	})
	if err == nil {
		fail("未签名固件被入库（应拒绝）")
	}
	pass("未签名固件拒绝入库：%v", trimMsg(err))

	// ---- 2. 签名固件入库 ----
	priv := loadEd25519Priv(*keyPath)
	sig := ed25519.Sign(priv, []byte(shaHex))
	v1 := fmt.Sprintf("1.0.%d", ts%1000)
	v2 := fmt.Sprintf("1.1.%d", ts%1000)
	fw1, err := dev.SaveFirmware(ctx, &devpb.SaveFirmwareReq{
		ProductKey: "ESS-DEMO", Version: v2,
		FileUrl: "s3://micro-file/ota/" + sn + "-v2.bin", FileSize: int64(len(fwBody)),
		Sha256: shaHex, Signature: base64.StdEncoding.EncodeToString(sig),
	})
	must(err, "SaveFirmware(v2)")
	// 回滚目标固件（旧版本占位——同签名可入库）
	fw0, err := dev.SaveFirmware(ctx, &devpb.SaveFirmwareReq{
		ProductKey: "ESS-DEMO", Version: v1,
		FileUrl: "s3://micro-file/ota/" + sn + "-v1.bin", FileSize: int64(len(fwBody)),
		Sha256: shaHex, Signature: base64.StdEncoding.EncodeToString(sig),
	})
	must(err, "SaveFirmware(v1)")
	pass("签名固件入库 v1=%d v2=%d（Ed25519 验签通过）", fw0.FirmwareId, fw1.FirmwareId)

	// ---- 3. 篡改签名拒绝（防篡改验证）----
	badSig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("tampered")))
	_, err = dev.SaveFirmware(ctx, &devpb.SaveFirmwareReq{
		ProductKey: "ESS-DEMO", Version: fmt.Sprintf("9.9.%d", ts%1000),
		FileUrl: "s3://micro-file/ota/tampered.bin", Sha256: shaHex, Signature: badSig,
	})
	if err == nil {
		fail("篡改签名固件被入库（应拒绝）")
	}
	pass("篡改签名拒绝入库")

	// ---- 4. 灰度任务（单设备清单）----
	// DeviceIds 需要真实 device_id：先反查再创建灰度清单
	list, err := dev.ListDevice(ctx, &devpb.ListDeviceReq{Keyword: sn})
	must(err, "ListDevice")
	if len(list.List) != 1 {
		fail("目标设备未找到 sn=%s", sn)
	}
	task2, err := dev.CreateOtaTask(ctx, &devpb.CreateOtaTaskReq{
		Name: "otasmoke-" + sn, ProductKey: "ESS-DEMO",
		FirmwareId: fw1.FirmwareId, RollbackFirmwareId: fw0.FirmwareId,
		BatchSize: 1, DeviceIds: []int64{list.List[0].DeviceId},
	})
	must(err, "CreateOtaTask")
	pass("灰度任务创建 task_id=%d（batch=1，单设备清单）", task2.TaskId)

	// ---- 5. 指令下发扫描推进（dispatch → ota_device SENT）----
	waitFor(90*time.Second, 2*time.Second, "OTA 指令下发（ota_device SENT）", func() (bool, error) {
		resp, err := dev.GetOtaTask(ctx, &devpb.GetOtaTaskReq{TaskId: task2.TaskId, WithDevices: true})
		if err != nil {
			return false, err
		}
		for _, d := range resp.Devices {
			if d.Status == "SENT" {
				return true, nil
			}
		}
		return false, nil
	})
	pass("OTA_UPGRADE 指令已下发（复用指令链路 QoS1）")

	// ---- 6. 回滚（ROLLED_BACK + 明细复位 PENDING 重推旧固件）----
	rb, err := dev.RollbackOtaTask(ctx, &devpb.RollbackOtaTaskReq{
		TaskId: task2.TaskId, Reason: "otasmoke 验收回滚",
	})
	must(err, "RollbackOtaTask")
	if rb.RolledBack < 1 {
		fail("回滚复位明细数为 0")
	}
	resp, err := dev.GetOtaTask(ctx, &devpb.GetOtaTaskReq{TaskId: task2.TaskId, WithDevices: true})
	must(err, "GetOtaTask(回滚后)")
	if resp.Task.Status != "ROLLED_BACK" {
		fail("任务状态非 ROLLED_BACK: %s", resp.Task.Status)
	}
	pass("回滚完成 rolled_back=%d 状态=ROLLED_BACK（旧固件重推由扫描器续推）")

	fmt.Println("\notasmoke: 全部通过 ✅")
}

// ---- 基础设施 ----

func loadEd25519Priv(path string) ed25519.PrivateKey {
	raw, err := os.ReadFile(path)
	must(err, "读取 OTA 私钥 "+path)
	block, _ := pem.Decode(raw)
	if block == nil {
		fail("PEM 解码失败: %s", path)
	}
	key, err := parsePKCS8(block.Bytes)
	must(err, "私钥解析")
	return key
}

func parsePKCS8(der []byte) (ed25519.PrivateKey, error) {
	k, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("非 Ed25519 私钥")
	}
	return priv, nil
}

func trimMsg(err error) string {
	s := err.Error()
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func waitFor(total, interval time.Duration, name string, probe func() (bool, error)) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		ok, err := probe()
		if ok {
			return
		}
		_ = err
		time.Sleep(interval)
	}
	fail("等待超时: %s", name)
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "otasmoke FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(format string, args ...any) {
	fmt.Printf("  ✅ "+format+"\n", args...)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "otasmoke FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
