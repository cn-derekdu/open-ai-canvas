package app

// SSE 解析失败现场的观测回归测试。
//
// 背景：线上出现过 "Agent 流式事件解析失败：invalid character 'i' after object key"，
// 因为错误里不含原始帧，无法判断脏数据是什么（上游中转把非 JSON 片段混进 event-stream）。
// 这些用例锁住"失败必须带出原始帧"这一观测契约，避免后续合并被静默改回。

import (
	"strings"
	"testing"
)

// TestStreamingAgentParserReportsRawFrameWithObjectKeyError 锁住生产事故的原始形态：
// 帧内出现「已解析出键、随后不是冒号」的畸形 JSON 时，报错里必须能看到脏数据本身。
func TestStreamingAgentParserReportsRawFrameWithObjectKeyError(t *testing.T) {
	parser := newStreamingAgentParser("chat-completion", func(string) {})
	// 上游把非 JSON 片段接到键后面，正是 "invalid character 'i' after object key" 的成因。
	parser.consume("text/event-stream", []byte("data: {\"type\":\"text\",\"index\" image_url}\n\n"))

	if parser.err == nil {
		t.Fatal("畸形帧必须让解析失败，而不是当成正常事件")
	}
	message := parser.err.Error()
	if !strings.Contains(message, "invalid character 'i' after object key") {
		t.Fatalf("应保留原始 JSON 报错以便定位，实际：%s", message)
	}
	if !strings.Contains(message, "原始帧=") || !strings.Contains(message, "image_url") {
		t.Fatalf("报错必须带出原始帧内容，实际：%s", message)
	}
	if _, err := parser.result(); err == nil {
		t.Fatal("result() 应把解析失败暴露给上层")
	}
}

// TestStreamingAgentParserReportsRawFrameForMultiLineMerge 覆盖多段 data: 被拼成一帧的场景：
// 拼接后会引入换行，原始帧必须以单行转义形式记录，否则日志会被截断成多行。
func TestStreamingAgentParserReportsRawFrameForMultiLineMerge(t *testing.T) {
	parser := newStreamingAgentParser("chat-completion", func(string) {})
	parser.consume("text/event-stream", []byte(
		"data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"第一镜\"}}]}\n"+
			"data: invalid_request_error: rate limit exceeded\n\n"))

	if parser.err == nil {
		t.Fatal("混入非 JSON 提示文本时必须失败")
	}
	message := parser.err.Error()
	if !strings.Contains(message, "原始帧=") || !strings.Contains(message, "invalid_request_error") {
		t.Fatalf("报错必须带出拼接后的原始帧，实际：%s", message)
	}
	if strings.Contains(message, "\n") {
		t.Fatalf("原始帧必须转义换行以保持日志单行，实际：%q", message)
	}
}

// TestStreamingAgentParserStillEmitsValidFrames 保证补诊断没有改变正常行为。
func TestStreamingAgentParserStillEmitsValidFrames(t *testing.T) {
	deltas := new(strings.Builder)
	parser := newStreamingAgentParser("chat-completion", func(delta string) { deltas.WriteString(delta) })
	parser.consume("text/event-stream", []byte(
		"data: {\"choices\":[{\"delta\":{\"content\":\"第一镜\"}}]}\n\n"+
			"data: {\"choices\":[{\"delta\":{\"content\":\"：远景\"}}]}\n\n"))
	parser.flush()

	result, err := parser.result()
	if err != nil {
		t.Fatalf("合法流不应报错：%v", err)
	}
	if deltas.String() != "第一镜：远景" {
		t.Fatalf("增量回调内容异常：%q", deltas.String())
	}
	if text, _ := result["text"].(string); !strings.Contains(text, "第一镜") {
		t.Fatalf("结果应带上正文，实际：%#v", result)
	}
}

// TestStreamingTextDeltaParserSkipsMalformedFrameAndKeepsEmitting 这条路径本来就跳过坏帧继续出字，
// 行为保持不变，但必须留下现场。
func TestStreamingTextDeltaParserSkipsMalformedFrameAndKeepsEmitting(t *testing.T) {
	deltas := new(strings.Builder)
	parser := newStreamingTextDeltaParser("chat-completion", func(delta string) { deltas.WriteString(delta) })
	parser.consume("text/event-stream", []byte(
		"data: {\"choices\":[{\"delta\":{\"content\" borked}}]}\n\n"+
			"data: {\"choices\":[{\"delta\":{\"content\":\"仍然出字\"}}]}\n\n"))
	parser.flush()

	if deltas.String() != "仍然出字" {
		t.Fatalf("坏帧应被跳过且不影响后续增量，实际：%q", deltas.String())
	}
}

// TestParseTextEventStreamReportsRawFrame 非流式累积路径同样要带出原始帧。
func TestParseTextEventStreamReportsRawFrame(t *testing.T) {
	_, err := parseTextEventStream([]byte("data: {\"choices\" broken payload}\n\n"), "chat-completion")
	if err == nil {
		t.Fatal("畸形帧必须失败")
	}
	message := err.Error()
	if !strings.Contains(message, "原始帧=") || !strings.Contains(message, "broken payload") {
		t.Fatalf("报错必须带出原始帧，实际：%s", message)
	}
}

// TestSSEFrameSnippetBoundsAndEscapes 直接锁住摘要助手的边界：短帧原样引用、长帧截断并标注原始长度。
func TestSSEFrameSnippetBoundsAndEscapes(t *testing.T) {
	short := sseFrameSnippet("  {\"ok\":true}\n  ")
	if short != `"{\"ok\":true}"` {
		t.Fatalf("短帧应去空白并原样引用，实际：%s", short)
	}

	long := sseFrameSnippet(strings.Repeat("a", 500))
	if !strings.Contains(long, "此处截断") || !strings.Contains(long, "原始 500 字节") {
		t.Fatalf("长帧应标注截断与原始字节数，实际：%s", long)
	}
	if strings.Contains(long, "\n") {
		t.Fatalf("摘要必须保持单行，实际：%q", long)
	}
	if len(long) > sseFrameSnippetLimit+64 {
		t.Fatalf("摘要长度应受限于 %d 字节，实际 %d", sseFrameSnippetLimit, len(long))
	}
}
