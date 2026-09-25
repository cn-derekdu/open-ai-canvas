package app

import (
	"strings"
	"testing"
)

// 生产现场（2026-09-25）：auto 模式下画布写入立即生效、不会创建审批卡，但工具结果里仍带着
// "批准后才会写入画布" 的审批文案，模型据此让用户去点击一张不存在的卡，对话停在原地。
func TestCloudAgentAppliedCanvasPreviewDropsApprovalSemantics(t *testing.T) {
	preview := cloudAgentCanvasApprovalPreview([]cloudAgentApprovalPreviewItem{
		{Operation: "update_node", NodeID: "image-1", Summary: "修改图片《S04》的提示词"},
		{Operation: "update_node", NodeID: "image-2", Summary: "修改图片《P01》的提示词"},
		{Operation: "update_node", NodeID: "image-3", Summary: "修改图片《P02》的提示词"},
	})
	if !strings.Contains(preview.Description, "批准后才会写入画布") {
		t.Fatalf("审批预览应保留审批语气: %q", preview.Description)
	}
	applied := cloudAgentAppliedCanvasPreview(preview)
	if applied.Title != "画布修改" {
		t.Fatalf("已生效预览不应再叫“确认画布修改”: %q", applied.Title)
	}
	// 标题与描述都不能再出现"等待批准/批准后才会写入"这类待办语气；
	// 只有明确否定形式（"不需要用户批准"）是允许且需要的。
	for _, forbidden := range []string{"确认", "准备", "批准后", "请批准"} {
		if strings.Contains(applied.Title+applied.Description, forbidden) {
			t.Fatalf("已生效预览不得残留审批语气 %q: %q / %q", forbidden, applied.Title, applied.Description)
		}
	}
	if !strings.Contains(applied.Description, "已修改 3 个节点") || !strings.Contains(applied.Description, "画布已更新") {
		t.Fatalf("已生效预览应说明实际写入结果: %q", applied.Description)
	}
	if !strings.Contains(applied.Description, "立即生效") {
		t.Fatalf("已生效预览应明确不需要用户批准: %q", applied.Description)
	}
	if len(applied.Items) != len(preview.Items) {
		t.Fatalf("改写后必须保留明细: %d != %d", len(applied.Items), len(preview.Items))
	}
}

func TestCloudAgentApprovalGatedCanvasPreviewOnlyKeepsWordingWhenGated(t *testing.T) {
	preview := cloudAgentCanvasApprovalPreview([]cloudAgentApprovalPreviewItem{{Operation: "update_node", NodeID: "n1"}})
	gated := &cloudAgentRuntime{Approval: &cloudAgentApproval{ID: "ap-1", Decision: "approve"}}
	if got := cloudAgentApprovalGatedCanvasPreview(gated, preview); got.Title != preview.Title {
		t.Fatalf("走过审批的写入应保留审批文案: %q", got.Title)
	}
	if got := cloudAgentApprovalGatedCanvasPreview(&cloudAgentRuntime{}, preview); got.Title != "画布修改" {
		t.Fatalf("无需审批的写入必须改成已生效文案: %q", got.Title)
	}
	if got := cloudAgentApprovalGatedCanvasPreview(nil, preview); got.Title != "画布修改" {
		t.Fatalf("缺少运行态时按无需审批处理: %q", got.Title)
	}
}

func TestCloudAgentRewriteUngatedPreviewRewritesToolResult(t *testing.T) {
	preview := cloudAgentCanvasApprovalPreview([]cloudAgentApprovalPreviewItem{{Operation: "update_node", NodeID: "n1"}})
	auto := &cloudAgentRuntime{}
	settled := &cloudAgentRuntime{Approval: &cloudAgentApproval{ID: "ap-1", Decision: "approve"}}

	asMap := cloudAgentRewriteUngatedPreview(auto, map[string]any{"summary": "已完成 1 项节点/连线操作", "preview": preview}).(map[string]any)
	rewritten, ok := asMap["preview"].(cloudAgentApprovalPreview)
	if !ok || rewritten.Title != "画布修改" {
		t.Fatalf("工具结果里的审批预览必须被改写: %#v", asMap["preview"])
	}
	if asMap["summary"] != "已完成 1 项节点/连线操作" {
		t.Fatalf("改写不得丢失其他字段: %#v", asMap)
	}
	// 审批路径（确实有卡）保持原样，且返回值不做无意义复制。
	gated := cloudAgentRewriteUngatedPreview(settled, map[string]any{"preview": preview}).(map[string]any)
	if gated["preview"].(cloudAgentApprovalPreview).Title != "确认画布修改" {
		t.Fatal("走过审批的工具结果不应被改写")
	}
	if _, stillString := cloudAgentRewriteUngatedPreview(auto, "plain").(string); !stillString {
		t.Fatal("非预览结构必须原样返回")
	}
	pointer := preview
	if got := cloudAgentRewriteUngatedPreview(auto, &pointer).(*cloudAgentApprovalPreview); got.Title != "画布修改" {
		t.Fatalf("指针形态也必须改写: %q", got.Title)
	}
}

func TestCloudAgentClaimsPendingApprovalMatchesProductionMessage(t *testing.T) {
	claims := []string{
		"当前停在第 2 步：等待你批准三条提示词写入。",
		"请在画布中的“确认画布修改”卡片点击批准。",
		"任务仍停在第 2 步：等待画布修改审批。",
		"批准后我会继续提交 S04、P01、P02 三项生图任务。",
	}
	for _, text := range claims {
		if !cloudAgentClaimsPendingApproval(text) {
			t.Fatalf("应识别为“自称在等审批”: %q", text)
		}
	}
	clean := []string{
		"",
		"三条提示词已写入画布，接下来提交生图。",
		"生图任务需要审批，已在提交时进入确认流程。",
		"画布已更新：S04、P01、P02 的提示词已就位。",
	}
	for _, text := range clean {
		if cloudAgentClaimsPendingApproval(text) {
			t.Fatalf("不应识别为“自称在等审批”: %q", text)
		}
	}
}

func TestCloudAgentShouldNudgeApprovalClaimGuards(t *testing.T) {
	claim := "请在画布中的“确认画布修改”卡片点击批准。"
	if !cloudAgentShouldNudgeApprovalClaim(&cloudAgentRuntime{}, claim) {
		t.Fatal("无待批准审批时的索要批准应当被纠正")
	}
	// 真有审批在等着，就不能纠正（模型没看错）。
	gated := &cloudAgentRuntime{Approval: &cloudAgentApproval{ID: "ap-1"}}
	if cloudAgentShouldNudgeApprovalClaim(gated, claim) {
		t.Fatal("存在待批准审批时不得纠正模型")
	}
	// 纠正次数用满后必须放行收尾，避免无限烧模型调用。
	exhausted := &cloudAgentRuntime{ApprovalClaimNudges: cloudAgentMaxApprovalClaimNudges}
	if cloudAgentShouldNudgeApprovalClaim(exhausted, claim) {
		t.Fatal("纠正次数用满后必须停止纠正")
	}
	if cloudAgentShouldNudgeApprovalClaim(nil, claim) {
		t.Fatal("缺少运行态时不得纠正")
	}
	if cloudAgentShouldNudgeApprovalClaim(&cloudAgentRuntime{}, "三条提示词已写入画布。") {
		t.Fatal("正常收尾不应被纠正")
	}
}

func TestCloudAgentApprovalClaimNudgeCarriesFacts(t *testing.T) {
	state := &cloudAgentRuntime{Request: CloudAgentRequest{PermissionMode: "auto"}, ActiveTaskID: "task-1"}
	message := cloudAgentApprovalClaimNudgeMessage(state)
	content, _ := message["content"].(string)
	if message["role"] != "user" || !strings.Contains(content, cloudAgentRuntimeContextMarker) {
		t.Fatalf("纠正消息必须是运行态消息: %#v", message)
	}
	for _, expected := range []string{"permissionMode=auto", "pendingTask=true", "approval_claim"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("纠正消息缺少事实 %q: %s", expected, content)
		}
	}
	if !strings.Contains(content, "不要等待") {
		t.Fatalf("纠正消息缺少行动要求: %s", content)
	}
}
