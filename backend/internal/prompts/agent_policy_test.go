package prompts

import (
	"strings"
	"testing"
)

func TestLoadAgentPoliciesUsesDocumentMetadata(t *testing.T) {
	system, media, err := LoadAgentPolicies()
	if err != nil {
		t.Fatal(err)
	}
	// system 版本高于上游：本仓库在系统策略里保留了「审批与权限模式」一节（审批误报修复的一半，
	// 用于让模型理解 approval_claim 与 auto 模式没有审批卡）。改这个数字必须同时改 md 的 front matter。
	if system.ID != "cloud-agent-system" || system.Version != 11 || media.ID != "cloud-agent-media" || media.Version != 4 {
		t.Fatalf("unexpected policy metadata: system=%+v media=%+v", system, media)
	}
	if strings.Contains(system.Text, "id: cloud-agent-system") || !strings.HasPrefix(system.Text, "# 影策 Cloud Agent") {
		t.Fatalf("metadata leaked into compiled policy body: %q", system.Text)
	}
	for _, phrase := range []string{
		"auto 会由服务端完成模型、能力、价格、预算和资源准入后直接提交",
		"request_approval 必须等待界面独立审批",
		"authorizedChargeMicrocredits / chargeLimitMicrocredits 是预授权或上限，不是实际消费",
	} {
		if !strings.Contains(media.Text, phrase) {
			t.Fatalf("media policy lost required contract phrase %q", phrase)
		}
	}
}

func TestParsePolicyDocumentRejectsInvalidMetadata(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"missing header", "# policy"},
		{"unclosed header", "---\nid: policy\nversion: 1\n# body"},
		{"missing id", "---\nversion: 1\n---\n# body"},
		{"invalid version", "---\nid: policy\nversion: latest\n---\n# body"},
		{"duplicate field", "---\nid: policy\nid: other\nversion: 1\n---\n# body"},
		{"unknown field", "---\nid: policy\nversion: 1\nowner: app\n---\n# body"},
		{"empty body", "---\nid: policy\nversion: 1\n---"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, _, err := parsePolicyDocument(test.raw); err == nil {
				t.Fatal("invalid policy document was accepted")
			}
		})
	}
}
