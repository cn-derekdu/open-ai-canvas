package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	cloudAgentMaxToolCalls         = 8
	cloudAgentMaxOutputBytes       = 32000
	cloudAgentRuntimeContextMarker = "【运行状态】"
	cloudAgentContextSourceKey     = "agentContextSource"
)

type cloudAgentRuntimeContextKind string

const (
	cloudAgentContextPlan               cloudAgentRuntimeContextKind = "plan_state"
	cloudAgentContextPendingPlan        cloudAgentRuntimeContextKind = "pending_plan"
	cloudAgentContextEmptyOutput        cloudAgentRuntimeContextKind = "empty_output"
	cloudAgentContextInvalidOutput      cloudAgentRuntimeContextKind = "invalid_output"
	cloudAgentContextTruncatedArguments cloudAgentRuntimeContextKind = "truncated_tool_arguments"
	cloudAgentContextApprovalClaim      cloudAgentRuntimeContextKind = "approval_claim"
)

// cloudAgentMaxApprovalClaimNudges 限制"模型自称在等审批"的纠正次数：每次纠正要多花一次
// 模型调用，纠正两次仍不改口说明是模型侧的问题，此时如实收尾比继续烧钱更合理。
const cloudAgentMaxApprovalClaimNudges = 2

// cloudAgentApprovalClaimPhrases 是"模型声称有审批在等"的判据。只看语气词容易误判，
// 所以要求同时出现审批语义与等待/索要动作：例如"任务仍停在第 2 步：等待画布修改审批"
// 或"请点击'确认画布修改'卡片中的'批准'"。
var cloudAgentApprovalClaimPhrases = []string{"等待", "等你", "点击", "点“", "点\"", "请批准", "批准后", "需要你"}

// cloudAgentClaimsPendingApproval 判断一段模型正文是否在声称"有审批等用户批准"。
func cloudAgentClaimsPendingApproval(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "审批") && !strings.Contains(trimmed, "批准") {
		return false
	}
	for _, phrase := range cloudAgentApprovalClaimPhrases {
		if strings.Contains(trimmed, phrase) {
			return true
		}
	}
	return false
}

// cloudAgentShouldNudgeApprovalClaim 决定是否就"自称在等审批"再推进一步：只有确实没有待批准的
// 审批、还没用满纠正次数、并且正文真的在索要批准时才纠正。
func cloudAgentShouldNudgeApprovalClaim(state *cloudAgentRuntime, text string) bool {
	if state == nil || state.Approval != nil {
		return false
	}
	if state.ApprovalClaimNudges >= cloudAgentMaxApprovalClaimNudges {
		return false
	}
	return cloudAgentClaimsPendingApproval(text)
}

// cloudAgentApprovalClaimNudgeMessage 让模型继续推进：本轮没有待批准的审批。
func cloudAgentApprovalClaimNudgeMessage(state *cloudAgentRuntime) map[string]any {
	permissionMode := ""
	hasPendingTask := false
	if state != nil {
		permissionMode = state.Request.PermissionMode
		hasPendingTask = state.ActiveTaskID != "" || state.MediaTaskID != ""
	}
	return cloudAgentRuntimeMessage(cloudAgentRuntimeContext{
		Kind: cloudAgentContextApprovalClaim,
		Detail: fmt.Sprintf(
			"本轮没有待批准的审批（permissionMode=%s，pendingTask=%t）。画布写入是否需要审批由权限模式决定：auto 下立即生效、没有卡片。不要等待或重复索要批准，也不要说审批卡已出现；请用已有结果继续未完成的工作，或如实说明阻塞原因。",
			permissionMode, hasPendingTask),
		LatestUserMessage: cloudAgentLatestUserInstruction(state.Canonical.Messages),
	})
}

// Only serializable fact fields belong here. Behavioral instructions live in
// prompts/agent-system-policy.md and share its version/hash contract.
type cloudAgentRuntimeContext struct {
	Source            string                       `json:"source"`
	Kind              cloudAgentRuntimeContextKind `json:"kind"`
	Items             []cloudAgentPlanItem         `json:"items,omitempty"`
	PendingTitle      string                       `json:"pendingTitle,omitempty"`
	LatestUserMessage string                       `json:"latestUserMessage,omitempty"`
	Detail            string                       `json:"detail,omitempty"`
	MaxToolCalls      int                          `json:"maxToolCalls,omitempty"`
	MaxOutputBytes    int                          `json:"maxOutputBytes,omitempty"`
}

func cloudAgentRuntimeMessage(context cloudAgentRuntimeContext) map[string]any {
	context.Source = "runtime"
	// The closed struct contains only strings, integers and serializable plan items.
	encoded, _ := json.Marshal(context)
	return map[string]any{
		"role": "user", "content": cloudAgentRuntimeContextMarker + string(encoded),
		cloudAgentContextSourceKey: "runtime",
	}
}

func cloudAgentLatestUserInstruction(messages []map[string]any) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		source := stringField(message, cloudAgentContextSourceKey)
		if stringField(message, "role") == "user" && (source == "" || source == "user_interjection") {
			return stringField(message, "content")
		}
	}
	return ""
}
