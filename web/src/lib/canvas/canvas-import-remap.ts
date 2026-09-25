import { CanvasNodeType, type CanvasNodeData, type CanvasNodeMetadata } from "@/types/canvas";

/** 导入包里某个文件重新上传到资源库后的映射结果。 */
export type ImportedMediaRemap = { storageKey: string; url: string };

export type ImportedNodeRemap = Partial<Pick<CanvasNodeMetadata, "storageKey" | "content" | "previewContent">>;

function isDeadBlob(value?: string) {
    return typeof value === "string" && value.startsWith("blob:");
}

/**
 * 导入画布 / 项目包时，包内文件会被重新上传到资源库，节点的 storageKey 随之重映射。
 *
 * 媒体节点的 content 只是一个可重新指向的地址，换成新地址没问题；但**文本节点的 content 是正文本身**
 * （TXT / Markdown 的内容），一旦被替换成 `/api/resources/<id>/file`，画布上就只剩一条路径：
 * storageKey 已指向新资源，正文除了残留在 prompt 里再无出处——用户看到的就是「导入的剧本只显示路径」。
 */
export function remapImportedNodeMedia(node: Pick<CanvasNodeData, "type" | "metadata">, mapped: ImportedMediaRemap | undefined): ImportedNodeRemap {
    const metadata = node.metadata || {};
    const next: ImportedNodeRemap = {};
    const storageKey = mapped ? mapped.storageKey : metadata.storageKey && !isDeadBlob(metadata.storageKey) ? metadata.storageKey : undefined;
    if (storageKey !== undefined) next.storageKey = storageKey;
    // 文本节点保留正文；失效的 blob 地址仍然清空（原本就取不回来了）。
    const resolve = (value?: string) => (mapped && node.type !== CanvasNodeType.Text ? mapped.url : isDeadBlob(value) ? "" : value);
    const content = resolve(metadata.content);
    if (content !== undefined) next.content = content;
    const previewContent = resolve(metadata.previewContent);
    if (previewContent !== undefined) next.previewContent = previewContent;
    return next;
}
