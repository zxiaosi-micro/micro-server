package main

import "encoding/json"

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

// extractInnerPayload 提取 msg.payload 字段原文（规则评估针对设备原始报文）。
func extractInnerPayload(msg string) string {
	var m struct {
		Payload string `json:"payload"`
	}
	if err := jsonUnmarshal(msg, &m); err == nil && m.Payload != "" {
		return m.Payload
	}
	return msg
}
