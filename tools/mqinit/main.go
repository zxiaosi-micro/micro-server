// mqinit · Kafka topic 预创建（E2：topic 先建后用，02 §8/§16）。
//
// 用法：go run ./tools/mqinit [-brokers 127.0.0.1:29092]
// 幂等：已存在的 topic 跳过；topic 清单以 micro-common/eventbus/topics.go 为唯一正本。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// topics 预创建清单（E2 先建后用）。
// ⚠ 与 micro-common/eventbus/topics.go 的 Topic 常量保持同步（升级 micro-common 后
// 改为直接调用 eventbus.Topics()）。
var topics = []string{
	// 订单/交易域
	"order_created", "order_paid", "order_cancelled", "order_pay_timeout",
	"stock_out", "stock_low", "payment_refunded", "order_return_approved", "warranty_started",
	// 基础三件套
	"notification_request", "audit_event", "export_requested",
	// IoT/资产域
	"iot_telemetry_raw", "alert_candidate", "device_activated", "shipment_signed",
	"cmd_ack", "cmd_failed", "ota_paused", "device_anomaly",
}

func main() {
	brokers := flag.String("brokers", "127.0.0.1:29092", "逗号分隔的 broker 地址")
	flag.Parse()

	addrs := splitAddrs(*brokers)
	log.Printf("mqinit: 预创建 %d 个 topic → %v", len(topics), addrs)

	// 分区数/副本数：dev 3 分区 1 副本；生产按吞吐规划（runbook 发布节调整）。
	var created, existed int
	for _, topic := range topics {
		conn, err := kafka.Dial("tcp", addrs[0])
		if err != nil {
			log.Fatalf("mqinit: 连接 broker 失败 %v: %v", addrs[0], err)
		}
		partitions, err := conn.ReadPartitions(topic)
		_ = conn.Close()
		if err == nil && len(partitions) > 0 {
			existed++
			continue
		}

		client := &kafka.Client{Addr: kafka.TCP(addrs...), Timeout: 10 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		resp, cerr := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{
			Topics: []kafka.TopicConfig{{
				Topic:             topic,
				NumPartitions:     3,
				ReplicationFactor: 1,
			}},
		})
		cancel()
		if cerr != nil {
			log.Printf("mqinit: 创建 %s 失败: %v", topic, cerr)
			continue
		}
		if e := resp.Errors[topic]; e != nil {
			if isTopicExists(e) {
				existed++
				continue
			}
			log.Printf("mqinit: 创建 %s 失败: %v", topic, e)
			continue
		}
		created++
		log.Printf("mqinit: 已创建 topic=%s partitions=3", topic)
	}
	fmt.Printf("mqinit 完成：新建 %d，已存在 %d，共 %d\n", created, existed, len(topics))
}

func splitAddrs(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func isTopicExists(err error) bool {
	te, ok := err.(kafka.Error)
	return ok && te == kafka.TopicAlreadyExists
}
