/**
 * 取媒体字节的统一边界。
 *
 * 浏览器原生 fetch 在网络层失败时只会给出英文「Failed to fetch」/「NetworkError」，
 * 对用户没有任何行动价值；而这类失败几乎总是 CORS 被拦、请求被扩展拦截或断网。
 * 媒体读取（资源文件、成片、音频、封面）都从这里走，把这些失败换成可行动的中文说明。
 *
 * 注意：CORS 拦截发生在浏览器内部，控制台仍会有浏览器自己打的一条错误，无法消除；
 * 这里保证的是**应用层提示**可读、可操作。
 */
const networkFailurePattern = /failed to fetch|networkerror|load failed|network request failed/i;

/** 是否是浏览器原生 fetch 的网络层失败（区别于有状态码的 HTTP 错误）。 */
export function isFetchNetworkFailure(cause: unknown) {
    return cause instanceof TypeError && networkFailurePattern.test(cause.message);
}

/** 把网络层失败换成可行动说明；不是这类失败时返回 null，由调用方原样抛出。 */
export function describeFetchNetworkFailure(cause: unknown, subject: string) {
    if (!isFetchNetworkFailure(cause)) return null;
    return `${subject}无法在浏览器里直接读取：请求被跨域策略或网络拦截（对象存储需要允许本站读取）。可先用节点上的「下载」逐个保存素材，或在存储设置里放通本站的跨域与下载域名。`;
}

/** 包一层 fetch：网络层失败换成可行动文案，其余错误原样抛给调用方。 */
export async function fetchWithReadableFailure(url: string, subject: string, init?: RequestInit): Promise<Response> {
    try {
        return await fetch(url, init);
    } catch (cause) {
        const readable = describeFetchNetworkFailure(cause, subject);
        if (readable) throw new Error(readable);
        throw cause;
    }
}

/** 直接取媒体字节：网络层失败与 HTTP 错误都换成可行动文案。 */
export async function fetchMediaBlob(url: string, subject: string, init?: RequestInit) {
    const response = await fetchWithReadableFailure(url, subject, init);
    if (!response.ok) throw new Error(`${subject}读取失败（HTTP ${response.status}）`);
    return response.blob();
}
