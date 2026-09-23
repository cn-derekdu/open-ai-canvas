import { LoaderCircle } from "lucide-react";

import type { SyncProjectProgress } from "@/stores/use-sync-progress-store";

/** 导入流程里无法给出文件计数的阶段：解压校验与保存（上传已完成）。 */
export type CanvasImportPhase = "reading" | "uploading" | "saving";

export type CanvasImportRun = {
    /** 当前正在导入的画布标题，用于让用户确认导的是哪一个。 */
    title: string;
    /** 当前第几个画布（从 1 开始）与本次压缩包内的画布总数。 */
    index: number;
    count: number;
    phase: CanvasImportPhase;
};

/**
 * 导入画布的常驻进度面板。
 *
 * 导入耗时几乎都在「上传媒体 → 登记素材 → 保存画布」上，而这段过程只被写进
 * useSyncProgressStore，此前仅画布编辑页（CanvasSyncStatus）会渲染它；在列表页点导入
 * 之后既没有百分比也没有阶段，用户无法判断是否还在进行。这里直接在触发导入的页面上
 * 呈现阶段、单个画布的文件计数与百分比，补齐这段空档。
 */
export function CanvasImportProgress({ run, progress }: { run: CanvasImportRun; progress?: SyncProjectProgress }) {
    const total = Math.max(0, progress?.total || 0);
    const completed = Math.min(Math.max(0, progress?.completed || 0), total);
    const percentage = total > 0 ? Math.round((completed / total) * 100) : 0;
    // 只有上传阶段才有可靠的分母；解压与保存阶段用不确定进度条，避免显示假百分比。
    const determinate = run.phase === "uploading" && total > 0;
    const detail =
        run.phase === "reading"
            ? "正在解压并校验压缩包"
            : run.phase === "saving"
              ? progress?.message || "正在保存画布"
              : `${progress?.message || "正在准备画布内容"}${total > 0 ? ` · ${completed}/${total}` : ""}`;

    return (
        <div
            role="status"
            aria-live="polite"
            className="fixed bottom-6 right-6 z-[var(--z-panel-floating)] w-[320px] rounded-xl border border-border bg-background/95 p-4 shadow-xl backdrop-blur"
        >
            <div className="flex items-start gap-3">
                <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg border border-border bg-muted text-muted-foreground">
                    <LoaderCircle className="size-4 animate-spin motion-reduce:animate-none" />
                </span>
                <span className="min-w-0 flex-1">
                    <span className="block text-xs font-semibold text-foreground">正在导入画布</span>
                    <span className="mt-0.5 block truncate text-[var(--fs-label)] text-muted-foreground" title={run.title}>
                        {run.title}
                    </span>
                </span>
                <span className="shrink-0 text-[var(--fs-tiny)] tabular-nums text-muted-foreground">
                    {run.index}/{run.count}
                </span>
            </div>
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-muted">
                {determinate ? (
                    <div className="h-full rounded-full bg-foreground/70 transition-all duration-300" style={{ width: `${percentage}%` }} />
                ) : (
                    <div className="h-full w-1/3 animate-pulse rounded-full bg-foreground/40 motion-reduce:animate-none" />
                )}
            </div>
            <div className="mt-2 flex items-center justify-between gap-2 text-[var(--fs-label)] text-muted-foreground">
                <span className="truncate">{detail}</span>
                {determinate ? <span className="shrink-0 tabular-nums">{percentage}%</span> : null}
            </div>
        </div>
    );
}
