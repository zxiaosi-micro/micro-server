// kafkaprobe · 主机侧 Kafka 探测（E3：容器内 CLI 连 advertised 地址会超时，宿主机侧诊断用）。
// 带消费组读 order_paid（模拟 kq 消费路径，诊断组消费是否可达）。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	fmt.Println("带消费组读 order_paid（group=probe-saga, first offset）…")
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"127.0.0.1:29092"},
		GroupID: "probe-saga",
		Topic:   "order_paid",
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait: 500 * time.Millisecond,
	})
	defer r.Close()
	r.SetOffset(kafka.FirstOffset)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			fmt.Printf("读取结束（%v）\n", err)
			return
		}
		fmt.Printf("off=%d key=%s\n", m.Offset, m.Key)
	}
}
