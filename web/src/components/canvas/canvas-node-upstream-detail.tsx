import { App, Modal, Spin } from "antd";
import { Search } from "lucide-react";
import { useState } from "react";

import { getAdminApiLog, listAdminApiLogs, type ApiCallLog } from "@/services/api/auth";
import { useUserStore } from "@/stores/use-user-store";

type UpstreamDetailTheme = {
    nodeText: string;
    nodeMuted: string;
    hoverBackground: string;
};

/**
 * 任务失败后按 taskId 反查上游调用记录，仅管理员可见。
 * 上游返回体不会进入面向用户的错误文案，这里提供受权限保护的排查入口：
 * 列表带出已脱敏的上游说明，原始报文按单条按需读取。
 */
export function CanvasNodeUpstreamDetail({ taskId, theme }: { taskId?: string; theme: UpstreamDetailTheme }) {
    const { message } = App.useApp();
    const isAdmin = useUserStore((state) => state.user?.role === "admin");
    const [open, setOpen] = useState(false);
    const [loading, setLoading] = useState(false);
    const [logs, setLogs] = useState<ApiCallLog[]>([]);
    const [bodies, setBodies] = useState<Record<string, ApiCallLog>>({});
    const [bodyLoadingId, setBodyLoadingId] = useState("");

    if (!isAdmin || !taskId) return null;

    const openDetail = async () => {
        setOpen(true);
        if (loading || logs.length > 0) return;
        setLoading(true);
        try {
            const result = await listAdminApiLogs({ taskId, pageSize: 20 });
            setLogs(result.logs);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取上游调用记录失败");
        } finally {
            setLoading(false);
        }
    };

    const loadBodies = async (logId: string) => {
        if (bodies[logId] || bodyLoadingId) return;
        setBodyLoadingId(logId);
        try {
            const result = await getAdminApiLog(logId);
            setBodies((current) => ({ ...current, [logId]: result.log }));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取原始报文失败");
        } finally {
            setBodyLoadingId("");
        }
    };

    return (
        <>
            <button
                type="button"
                className="inline-flex h-8 items-center gap-1.5 rounded-[var(--r-md)] px-3 text-xs font-medium transition-colors"
                style={{ background: theme.hoverBackground, color: theme.nodeMuted }}
                onClick={(event) => {
                    event.stopPropagation();
                    void openDetail();
                }}
                onMouseDown={(event) => event.stopPropagation()}
            >
                <Search className="size-3.5" />
                查看上游详情
            </button>
            <Modal
                open={open}
                onCancel={() => setOpen(false)}
                footer={null}
                width={780}
                title="上游调用详情（仅管理员可见）"
                destroyOnHidden
                styles={{ body: { maxHeight: "62vh", overflowY: "auto" } }}
            >
                {loading ? (
                    <div className="flex justify-center py-10">
                        <Spin />
                    </div>
                ) : logs.length === 0 ? (
                    <p className="py-6 text-center text-sm text-foreground/60">这条任务没有留下上游调用记录（可能未发出请求）。</p>
                ) : (
                    <div className="space-y-3">
                        {logs.map((log) => {
                            const body = bodies[log.id];
                            return (
                                <div key={log.id} className="rounded-lg border border-border/70 p-3">
                                    <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                                        <span className="font-medium">
                                            {log.model || "未识别模型"} · {log.path}
                                        </span>
                                        <span className={log.statusCode >= 400 ? "text-red-500" : "text-foreground/60"}>HTTP {log.statusCode || "—"}</span>
                                    </div>
                                    <div className="mt-1 text-xs text-foreground/50">
                                        {new Date(log.createdAt).toLocaleString()} · {log.upstreamUrl || "—"}
                                    </div>
                                    <div className="mt-2 whitespace-pre-wrap break-words text-xs leading-5 text-foreground/80">{log.error || "上游未返回错误详情"}</div>
                                    <div className="mt-2">
                                        {body ? (
                                            <pre className="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-foreground/5 p-2 text-xs leading-5">{body.responseBody || body.requestBody || "无原始报文"}</pre>
                                        ) : (
                                            <button
                                                type="button"
                                                className="text-xs text-foreground/50 underline-offset-2 hover:underline"
                                                onClick={() => void loadBodies(log.id)}
                                            >
                                                {bodyLoadingId === log.id ? "读取中…" : "查看原始报文"}
                                            </button>
                                        )}
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                )}
            </Modal>
        </>
    );
}
