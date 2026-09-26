import { afterEach, describe, expect, test } from "bun:test";

import { describeFetchNetworkFailure, fetchMediaBlob, fetchWithReadableFailure, isFetchNetworkFailure } from "@/services/media-fetch";

const originalFetch = globalThis.fetch;

afterEach(() => {
    globalThis.fetch = originalFetch;
});

describe("isFetchNetworkFailure", () => {
    test("识别浏览器原生的网络层失败", () => {
        expect(isFetchNetworkFailure(new TypeError("Failed to fetch"))).toBe(true);
        expect(isFetchNetworkFailure(new TypeError("NetworkError when attempting to fetch resource."))).toBe(true);
        expect(isFetchNetworkFailure(new TypeError("Load failed"))).toBe(true);
    });

    test("不把普通的 Error 或业务错误当成网络失败", () => {
        expect(isFetchNetworkFailure(new Error("Failed to fetch"))).toBe(false); // 必须是 TypeError
        expect(isFetchNetworkFailure(new TypeError("资源读取失败（HTTP 405）"))).toBe(false);
        expect(isFetchNetworkFailure(undefined)).toBe(false);
    });
});

describe("describeFetchNetworkFailure", () => {
    test("把 Failed to fetch 换成可行动说明", () => {
        const text = describeFetchNetworkFailure(new TypeError("Failed to fetch"), "资源");
        expect(text).toContain("资源");
        expect(text).toContain("跨域");
        expect(text).toContain("「下载」"); // 明确给出绕过办法
    });

    test("非网络失败返回 null，由调用方原样抛出", () => {
        expect(describeFetchNetworkFailure(new Error("boom"), "资源")).toBeNull();
    });
});

describe("fetchWithReadableFailure", () => {
    test("网络层失败抛可行动文案，而不是英文原文", async () => {
        globalThis.fetch = (() => Promise.reject(new TypeError("Failed to fetch"))) as typeof fetch;
        await expect(fetchWithReadableFailure("https://oss.example/x.png", "资源")).rejects.toThrow(/跨域策略或网络拦截/);
    });

    test("其余错误原样抛出", async () => {
        const abort = new DOMException("请求已取消", "AbortError");
        globalThis.fetch = (() => Promise.reject(abort)) as typeof fetch;
        await expect(fetchWithReadableFailure("https://oss.example/x.png", "资源")).rejects.toThrow("请求已取消");
    });
});

describe("fetchMediaBlob", () => {
    test("成功时返回 blob", async () => {
        globalThis.fetch = (async () => new Response(new Blob(["abc"]), { status: 200 })) as unknown as typeof fetch;
        const blob = await fetchMediaBlob("https://oss.example/x.png", "资源");
        expect(blob.size).toBe(3);
    });

    test("HTTP 错误带上状态码", async () => {
        globalThis.fetch = (async () => new Response("no", { status: 405 })) as unknown as typeof fetch;
        await expect(fetchMediaBlob("https://oss.example/x.png", "资源")).rejects.toThrow("资源读取失败（HTTP 405）");
    });

    test("网络层失败抛可行动文案", async () => {
        globalThis.fetch = (() => Promise.reject(new TypeError("Failed to fetch"))) as typeof fetch;
        await expect(fetchMediaBlob("https://oss.example/x.png", "成片")).rejects.toThrow(/成片无法在浏览器里直接读取/);
    });
});
