package app

import "testing"

// TestSkipRemainingCallsDetachesInFlightMediaTask 锁住 2026-09-27 线上事故的根因：
// 同一批里先 generate_media（任务刚提交、还在出片，MediaTaskID 已挂）、紧接着 ask_user
// 结束本轮。ask_user 分支会 skipRemainingCloudAgentCalls 把游标推到本批末尾，此时
// MediaTaskID 不再挂在"当前调用"上，检查点校验判定 "media task is not attached to current
// call" 并拒绝保存 —— 整轮被误报成"Agent 上下文或执行记录超过安全限制"而中止。
// 修复要求在推进游标前解绑未出片的媒体任务（任务本身不取消，成片照常进任务中心）。
func TestSkipRemainingCallsDetachesInFlightMediaTask(t *testing.T) {
	state := cloudAgentPairingState(
		cloudAgentPairingCall("call-media", "generate_media", `{"nodeId":"node-1"}`),
		cloudAgentPairingCall("call-ask", "ask_user", `{}`),
		cloudAgentPairingCall("call-later", "canvas_get_state", `{}`),
	)
	// 游标停在 ask_user 上（它的 tool 回执已记录），本批它后面还有未执行的调用；
	// 媒体任务仍在出片。
	state.CallIndex = 1
	state.MediaTaskID = "task-1"
	state.TaskIDs = []string{"run-1", "task-1"}

	skipRemainingCloudAgentCalls("run-1", state)

	if state.MediaTaskID != "" {
		t.Fatal("在出片的媒体任务没有解绑：检查点会以 media task is not attached to current call 被拒")
	}
	// 修复前这里必然为真：解绑缺失时，只要游标已越界，保存就被拒。
	if state.MediaTaskID != "" && state.CallIndex >= len(state.Calls) {
		t.Fatal("检查点仍会被拒：媒体任务挂在越界的游标上")
	}
	if !cloudAgentContainsString(state.TaskIDs, "task-1") {
		t.Fatal("解绑不应把任务从记录里删掉（任务中心与画布回写仍依赖它）")
	}
	detached := ""
	for _, event := range state.Events {
		if event.Type == "media_task_detached" {
			detached, _ = event.Payload["taskId"].(string)
		}
	}
	if detached != "task-1" {
		t.Fatalf("解绑没有留下可追溯事件，payload=%q", detached)
	}
}
