import { describe, expect, test } from "bun:test";

import { imageRequiresCorsApproval } from "../src/lib/canvas/canvas-image-data";

// 该模块按 window.location 判定跨源，测试进程没有 DOM，这里给出最小替身
// （必须同时提供 origin，否则同源地址会被判成跨源）。
const runtime = globalThis as unknown as { window?: { location: { href: string; origin: string } } };
runtime.window = { location: { href: "https://routerbox.cc/canvas/7UFoWgVZj6vrM6MO1Kvh9", origin: "https://routerbox.cc" } };

describe("canvas image export cross-origin guard", () => {
    // 回归：平台资源端点字面同源，但会 307 到对象存储，实际字节是跨源的。
    // 漏判时图片以 no-CORS 模式加载成功，画布被 taint，toDataURL 抛
    // 「Tainted canvases may not be exported」。
    test("treats the resource file endpoint as cross-origin", () => {
        expect(imageRequiresCorsApproval("/api/resources/4895f998b8300dcf26f7b204e1cf9dec/file")).toBe(true);
        expect(imageRequiresCorsApproval("https://routerbox.cc/api/resources/abc123/file")).toBe(true);
        expect(imageRequiresCorsApproval("/api/resources/abc123/file?variant=playback")).toBe(true);
    });

    test("keeps literal cross-origin http addresses requested with CORS", () => {
        expect(imageRequiresCorsApproval("https://cdn.example.com/poster.png")).toBe(true);
    });

    test("leaves same-origin assets, data and blob URLs untouched", () => {
        expect(imageRequiresCorsApproval("/assets/logo.png")).toBe(false);
        expect(imageRequiresCorsApproval("data:image/png;base64,AAAA")).toBe(false);
        expect(imageRequiresCorsApproval("blob:https://routerbox.cc/2f0a4c")).toBe(false);
        expect(imageRequiresCorsApproval("/api/resources/abc123/thumb")).toBe(false);
    });
});
