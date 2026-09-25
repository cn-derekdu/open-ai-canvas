import { describe, expect, test } from "bun:test";

import { remapImportedNodeMedia } from "@/lib/canvas/canvas-import-remap";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

const mapped = { storageKey: "resource:c501", url: "/api/resources/c501/file" };

function node(type: CanvasNodeType, metadata: CanvasNodeData["metadata"]): Pick<CanvasNodeData, "type" | "metadata"> {
    return { type, metadata };
}

describe("remapImportedNodeMedia", () => {
    test("媒体节点换成新的资源地址", () => {
        const result = remapImportedNodeMedia(node(CanvasNodeType.Image, { storageKey: "image:old", content: "blob:dead", previewContent: "blob:dead" }), mapped);
        expect(result).toEqual({ storageKey: "resource:c501", content: "/api/resources/c501/file", previewContent: "/api/resources/c501/file" });
    });

    test("文本节点保留正文，绝不替换成资源路径", () => {
        const script = "# 《停电的晚上》\n\n> 千禧年 · 川渝丘陵农村";
        const result = remapImportedNodeMedia(node(CanvasNodeType.Text, { storageKey: "file:old", content: script, prompt: script }), mapped);
        expect(result.storageKey).toBe("resource:c501");
        expect(result.content).toBe(script);
    });

    test("文本节点的失效 blob 正文仍然清空（避免留下取不回来的地址）", () => {
        const result = remapImportedNodeMedia(node(CanvasNodeType.Text, { storageKey: "file:old", content: "blob:dead" }), mapped);
        expect(result.content).toBe("");
    });

    test("没有重映射时保持原样，且不引入多余的键", () => {
        const result = remapImportedNodeMedia(node(CanvasNodeType.Text, { storageKey: "file:old", content: "正文" }), undefined);
        expect(result).toEqual({ storageKey: "file:old", content: "正文" });
        expect("previewContent" in result).toBe(false);
    });

    test("未失效的旧 key 在没有重映射时保留", () => {
        expect(remapImportedNodeMedia(node(CanvasNodeType.Video, { storageKey: "resource:keep" }), undefined).storageKey).toBe("resource:keep");
    });

    test("失效的旧 key 在没有重映射时被丢弃", () => {
        expect(remapImportedNodeMedia(node(CanvasNodeType.Video, { storageKey: "blob:dead" }), undefined).storageKey).toBeUndefined();
    });
});
