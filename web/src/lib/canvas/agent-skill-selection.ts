/**
 * Agent 会话里的技能选择必须按「当前技能库」收敛。
 *
 * 服务端只接受已安装（`isAdded`）且启用（`status === 1`）的技能，任何一个不满足就会整单拒绝
 * （400 `只能使用用户技能库中已安装且启用的技能`）。而技能随时可能被取消安装、下架或换账号，
 * 会话记录里却仍留着旧的选择——此时如果不收敛，这个会话会永久发不出消息，界面只显示 400。
 */
export type AgentSkillSelection = {
    /** 过滤后可以直接提交的技能 ID（保持原顺序、去重）。 */
    ids: string[];
    /** 被剔除的失效技能 ID，用于明确告知用户而不是静默丢弃。 */
    dropped: string[];
};

export function pruneAgentSkillSelection(requested: readonly string[], installed: ReadonlySet<string>): AgentSkillSelection {
    const ids: string[] = [];
    const dropped: string[] = [];
    for (const raw of requested) {
        const id = raw.trim();
        if (!id) continue;
        if (installed.has(id)) {
            if (!ids.includes(id)) ids.push(id);
            continue;
        }
        if (!dropped.includes(id)) dropped.push(id);
    }
    return { ids, dropped };
}

/** 技能库尚未读到时的收敛结果：返回原值，避免把用户的选择清空。 */
export function deferAgentSkillSelection(requested: readonly string[]): AgentSkillSelection {
    return { ids: [...new Set(requested.map((id) => id.trim()).filter(Boolean))], dropped: [] };
}
