package app

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

// 2026-09-27 线上事故回归（auto 模式 + 媒体生成，7 次全中）：
//
// auto 模式提交媒体任务后，成片还在出片时调度器会再来一拍。这一拍原本会重新进入
// "写工具准入/提交"分支（`advanceCloudAgentTool` 的 cloudAgentWrite+mediaTool 分支），
// 在 `cloudAgentMediaError(..., submitted=false, ...)` 里把游标推进到批次末尾，而
// `MediaTaskID` 仍挂着 —— 检查点校验以 "media task is not attached to current call"
// 拒绝保存，整轮被 terminate 成 failed（对外误报"Agent 上下文或执行记录超过安全限制"），
// 并且把**正在出片的成片一起取消**。
//
// 正确行为：只要 MediaTaskID 非空，媒体调用这一拍必须回到 `advanceCloudAgentMedia`
// 的"等待/完成"分支（审批模式一直是这样，auto 模式缺失这条分派）。
func TestAutoMediaInFlightWaitsWithoutCheckpointRejection(t *testing.T) {
	s, _, args := agentMediaFixture(t)
	run, _ := agentMediaRun(t, s, args, "auto")

	// 第一拍：auto 直接准入并提交媒体任务。
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatalf("auto 提交媒体任务失败: %v", err)
	}
	run, err := s.repo.CloudAgent("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := cloudAgentDecode(run)
	if err != nil {
		t.Fatal(err)
	}
	if state.MediaTaskID == "" {
		t.Fatalf("auto 模式没有提交媒体任务: status=%s events=%+v", run.Status, state.Events)
	}
	inFlight := state.MediaTaskID

	// 第二拍：成片仍在出片（任务保持 queued/running），这一拍不得判死本轮。
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatalf("出片中的推进返回错误: %v", err)
	}
	run, err = s.repo.CloudAgent("user", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "running" {
		t.Fatalf("出片中的一轮被误判为终态: status=%s failure=%q events=%+v", run.Status, run.FailureMessage, state.Events)
	}
	if strings.Contains(run.FailureMessage, "安全限制") {
		t.Fatalf("仍命中检查点被拒的统一文案: %q", run.FailureMessage)
	}
	state, err = cloudAgentDecode(run)
	if err != nil {
		t.Fatal(err)
	}
	if state.MediaTaskID != inFlight {
		t.Fatalf("出片中的媒体任务被解绑或替换: want=%s got=%s", inFlight, state.MediaTaskID)
	}
	task, err := s.repo.TaskForUser("user", inFlight)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status == model.TaskStatusCancelled {
		t.Fatal("出片中的成片被取消了（用户会同时丢掉本轮和这张图）")
	}
}

// 同一条不变量在"多个媒体调用"下的边界：出片中不允许出现第二个在飞媒体任务。
func TestAutoMediaInFlightKeepsSingleTaskID(t *testing.T) {
	s, _, args := agentMediaFixture(t)
	run, _ := agentMediaRun(t, s, args, "auto")
	if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
		t.Fatalf("auto 提交媒体任务失败: %v", err)
	}
	run, _ = s.repo.CloudAgent("user", run.ID)
	state, err := cloudAgentDecode(run)
	if err != nil {
		t.Fatal(err)
	}
	first := state.MediaTaskID
	if first == "" {
		t.Fatal("auto 模式没有提交媒体任务")
	}
	mediaTasks := 0
	for _, id := range state.TaskIDs {
		task, err := s.repo.TaskForUser("user", id)
		if err != nil || task.Operation != "text_to_video" && task.Type != "canvas_video" {
			continue
		}
		mediaTasks++
	}
	if mediaTasks > 1 {
		t.Fatalf("出片中出现重复媒体任务: %d", mediaTasks)
	}
	// 再走两拍，任务 ID 必须保持不变（既不能丢，也不能换成新任务重复扣费）。
	for i := 0; i < 2; i++ {
		if err := s.advanceCloudAgentByID("user", run.ID); err != nil {
			t.Fatalf("第 %d 次推进失败: %v", i+1, err)
		}
		run, _ = s.repo.CloudAgent("user", run.ID)
		if run.Status != "running" {
			t.Fatalf("第 %d 次推进把本轮判死: status=%s failure=%q", i+1, run.Status, run.FailureMessage)
		}
		state, _ = cloudAgentDecode(run)
		if state.MediaTaskID != first {
			t.Fatalf("第 %d 次推进改变了在飞媒体任务: want=%s got=%s", i+1, first, state.MediaTaskID)
		}
	}
}
