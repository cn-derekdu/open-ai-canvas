import { describe, expect, test } from "bun:test";

import { deferAgentSkillSelection, pruneAgentSkillSelection } from "@/lib/canvas/agent-skill-selection";

describe("pruneAgentSkillSelection", () => {
    test("剔掉不在技能库中的技能，并保留原顺序", () => {
        const installed = new Set(["a", "c"]);
        expect(pruneAgentSkillSelection(["a", "b", "c"], installed)).toEqual({ ids: ["a", "c"], dropped: ["b"] });
    });

    test("去重且忽略空白项", () => {
        const installed = new Set(["a"]);
        expect(pruneAgentSkillSelection(["a", " a ", "", "  ", "a"], installed)).toEqual({ ids: ["a"], dropped: [] });
    });

    test("失效技能只报告一次", () => {
        const installed = new Set<string>();
        expect(pruneAgentSkillSelection(["x", "x", "y"], installed)).toEqual({ ids: [], dropped: ["x", "y"] });
    });

    test("全部可用时原样返回", () => {
        const installed = new Set(["a", "b"]);
        expect(pruneAgentSkillSelection(["b", "a"], installed)).toEqual({ ids: ["b", "a"], dropped: [] });
    });

    test("技能库为空集合时全部判为失效（调用方需先用 defer 挡住未加载场景）", () => {
        expect(pruneAgentSkillSelection(["a"], new Set<string>())).toEqual({ ids: [], dropped: ["a"] });
    });
});

describe("deferAgentSkillSelection", () => {
    test("技能库未就绪时不丢选择，只做去重与清洗", () => {
        expect(deferAgentSkillSelection(["a", " a ", "b", ""])).toEqual({ ids: ["a", "b"], dropped: [] });
    });
});
