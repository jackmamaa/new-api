#!/usr/bin/env python3
"""
Veo 3.1 视频生成测试脚本
通过 new-api 网关调用 Gemini Veo 视频生成接口

支持的功能：
- 文生视频 (text-to-video)
- 图生视频 (image-to-video)
- 首尾帧生视频 (first + last frame interpolation)
- 参考图生视频 (reference images)
- 任务状态轮询

用法:
  python test_generation_video.py
"""

import base64
import json
import os
import sys
import time
import urllib.request
import urllib.error

# ============================================================
# 配置
# ============================================================

BASE_URL = os.environ.get("NEWAPI_BASE_URL", "https://new.sysrv.net")
API_KEY = os.environ.get("NEWAPI_API_KEY", "sk-MQC98iHFxO0ZxGtLFaraSemawXbttnCKd1BWsaQT8PS6Rl1g")
MODEL = "veo-3.1-generate-preview"

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
}


# ============================================================
# 工具函数
# ============================================================

def load_image_base64(path: str) -> str:
    """读取图片文件并返回 base64 编码字符串"""
    with open(path, "rb") as f:
        return base64.b64encode(f.read()).decode("utf-8")


def load_image_data_uri(path: str) -> str:
    """读取图片文件并返回 data URI"""
    ext = os.path.splitext(path)[1].lower().lstrip(".")
    mime_map = {"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "webp": "image/webp"}
    mime = mime_map.get(ext, "application/octet-stream")
    b64 = load_image_base64(path)
    return f"data:{mime};base64,{b64}"


def api_request(method: str, path: str, body: dict | None = None) -> dict:
    """发送 API 请求"""
    url = f"{BASE_URL}{path}"
    data = json.dumps(body).encode("utf-8") if body else None
    req = urllib.request.Request(url, data=data, headers=HEADERS, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        print(f"  HTTP {e.code}: {body}")
        return {"error": body}


def poll_task(task_id: str, interval: int = 10, timeout: int = 600) -> dict:
    """轮询任务状态直到完成或超时"""
    print(f"\n  轮询任务: {task_id}")
    start = time.time()
    while True:
        elapsed = time.time() - start
        if elapsed > timeout:
            print(f"  ⚠ 超时 ({timeout}s)")
            return {"status": "timeout"}

        result = api_request("GET", f"/v1/video/generations/{task_id}")
        data = result.get("data", result)
        status = data.get("status", "unknown")
        progress = data.get("progress", "N/A")
        print(f"  [{elapsed:.0f}s] status={status}, progress={progress}")

        if status in ("succeeded", "SUCCESS", "completed"):
            return data
        if status in ("failed", "FAILURE", "failure"):
            return data

        time.sleep(interval)


# ============================================================
# 示例 1: 文生视频 (Text-to-Video)
# ============================================================

def example_text_to_video():
    print("=" * 60)
    print("示例 1: 文生视频 (Text-to-Video)")
    print("=" * 60)

    body = {
        "model": MODEL,
        "prompt": "A cinematic drone shot flying over a misty mountain lake at sunrise, "
                  "with golden light reflecting on calm water, birds flying in the distance.",
        "metadata": {
            "aspectRatio": "16:9",
            "negativePrompt": "blurry, low quality, distorted",
        },
    }

    print("  请求体:")
    print(f"    {json.dumps(body, indent=2, ensure_ascii=False)}")

    result = api_request("POST", "/v1/video/generations", body)
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")

    task_id = result.get("id") or result.get("task_id")
    if task_id:
        final = poll_task(task_id)
        print(f"  最终结果: {json.dumps(final, indent=2, ensure_ascii=False)}")


# ============================================================
# 示例 2: 图生视频 (Image-to-Video) — 通过 metadata.inlineData
# ============================================================

def example_image_to_video(first_frame_path: str | None = None):
    print("\n" + "=" * 60)
    print("示例 2: 图生视频 (Image-to-Video) — metadata.inlineData")
    print("=" * 60)

    if first_frame_path and os.path.exists(first_frame_path):
        b64 = load_image_base64(first_frame_path)
    else:
        # 使用占位 base64（实际使用时替换为真实图片）
        print("  ⚠ 未提供图片，使用占位 base64")
        b64 = "iVBORw0KGgo..."  # 占位

    body = {
        "model": MODEL,
        "prompt": "A cat playing piano in a cozy room, soft lighting, cinematic.",
        "metadata": {
            "image": {
                "inlineData": {
                    "mimeType": "image/png",
                    "data": b64,
                }
            }
        },
    }

    print("  请求体 (metadata.image.inlineData):")
    print(f"    model: {body['model']}")
    print(f"    prompt: {body['prompt']}")
    print(f"    metadata.image.inlineData.data: {b64[:30]}...")

    result = api_request("POST", "/v1/video/generations", body)
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")

    task_id = result.get("id") or result.get("task_id")
    if task_id:
        final = poll_task(task_id)
        print(f"  最终结果: {json.dumps(final, indent=2, ensure_ascii=False)}")


# ============================================================
# 示例 3: 首尾帧生视频 (First + Last Frame Interpolation)
# ============================================================

def example_first_last_frame(first_path: str | None = None, last_path: str | None = None):
    print("\n" + "=" * 60)
    print("示例 3: 首尾帧生视频 (First + Last Frame)")
    print("=" * 60)

    if first_path and os.path.exists(first_path):
        first_b64 = load_image_base64(first_path)
    else:
        print("  ⚠ 未提供首帧图片，使用占位 base64")
        first_b64 = "iVBORw0KGgo..."  # 占位

    if last_path and os.path.exists(last_path):
        last_b64 = load_image_base64(last_path)
    else:
        print("  ⚠ 未提供尾帧图片，使用占位 base64")
        last_b64 = "iVBORw0KGgo..."  # 占位

    body = {
        "model": MODEL,
        "prompt": "A cinematic, haunting video. A ghostly woman with long white hair "
                  "and a flowing dress swings gently on a rope swing beneath a massive, "
                  "gnarled tree in a foggy, moonlit clearing. The fog thickens and "
                  "swirls around her, and she slowly fades away, vanishing completely.",
        "metadata": {
            "image": {
                "inlineData": {
                    "mimeType": "image/png",
                    "data": first_b64,
                }
            },
            "lastFrame": {
                "inlineData": {
                    "mimeType": "image/png",
                    "data": last_b64,
                }
            }
        },
    }

    print("  请求体:")
    print(f"    model: {body['model']}")
    print(f"    prompt: {body['prompt'][:80]}...")
    print(f"    metadata.image.inlineData.data: {first_b64[:30]}...")
    print(f"    metadata.lastFrame.inlineData.data: {last_b64[:30]}...")

    result = api_request("POST", "/v1/video/generations", body)
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")

    task_id = result.get("id") or result.get("task_id")
    if task_id:
        final = poll_task(task_id)
        print(f"  最终结果: {json.dumps(final, indent=2, ensure_ascii=False)}")


# ============================================================
# 示例 4: 首尾帧 — 使用 data URI 简化格式
# ============================================================

def example_first_last_frame_data_uri(first_path: str | None = None, last_path: str | None = None):
    print("\n" + "=" * 60)
    print("示例 4: 首尾帧 — data URI 简化格式")
    print("=" * 60)

    if first_path and os.path.exists(first_path):
        first_uri = load_image_data_uri(first_path)
    else:
        first_uri = "data:image/png;base64,iVBORw0KGgo..."

    if last_path and os.path.exists(last_path):
        last_uri = load_image_data_uri(last_path)
    else:
        last_uri = "data:image/png;base64,iVBORw0KGgo..."

    body = {
        "model": MODEL,
        "prompt": "A ginger cat driving a red convertible sports car along the "
                  "French Riviera coast, then the car launches off a cliff.",
        "metadata": {
            "image": first_uri,
            "lastFrame": last_uri,
        },
    }

    print("  请求体 (metadata 直接传 data URI 字符串):")
    print(f"    model: {body['model']}")
    print(f"    prompt: {body['prompt'][:80]}...")
    print(f"    metadata.image: {first_uri[:60]}...")
    print(f"    metadata.lastFrame: {last_uri[:60]}...")

    result = api_request("POST", "/v1/video/generations", body)
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")

    task_id = result.get("id") or result.get("task_id")
    if task_id:
        final = poll_task(task_id)
        print(f"  最终结果: {json.dumps(final, indent=2, ensure_ascii=False)}")


# ============================================================
# 示例 5: 参考图生视频 (Reference Images)
# ============================================================

def example_reference_images(ref_paths: list[str] | None = None):
    print("\n" + "=" * 60)
    print("示例 5: 参考图生视频 (Reference Images)")
    print("=" * 60)

    refs = []
    if ref_paths:
        for p in ref_paths:
            if os.path.exists(p):
                refs.append({
                    "image": {
                        "inlineData": {
                            "mimeType": "image/png",
                            "data": load_image_base64(p),
                        }
                    },
                    "referenceType": "asset",
                })
    if not refs:
        print("  ⚠ 未提供参考图，使用占位数据")
        refs = [
            {
                "image": {"inlineData": {"mimeType": "image/png", "data": "iVBORw0KGgo..."}},
                "referenceType": "asset",
            }
        ]

    body = {
        "model": MODEL,
        "prompt": "A high-fashion flamingo dress with cascading pink and magenta feathers, "
                  "worn by a beautiful woman with dark hair and warm brown eyes, "
                  "wearing quirky pink heart-shaped sunglasses.",
        "size": "1280x720",
        "duration": 8,
        "metadata": {
            "referenceImages": refs,
            "resolution": "720p",
            "durationSeconds": 8,
        },
    }

    print("  请求体:")
    print(f"    model: {body['model']}")
    print(f"    prompt: {body['prompt'][:80]}...")
    print(f"    metadata.referenceImages: {len(refs)} 张参考图")

    result = api_request("POST", "/v1/video/generations", body)
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")

    task_id = result.get("id") or result.get("task_id")
    if task_id:
        final = poll_task(task_id)
        print(f"  最终结果: {json.dumps(final, indent=2, ensure_ascii=False)}")


# ============================================================
# 示例 6: 仅查询已有任务
# ============================================================

def example_query_task(task_id: str):
    print("\n" + "=" * 60)
    print(f"示例 6: 查询任务 {task_id}")
    print("=" * 60)

    result = api_request("GET", f"/v1/video/generations/{task_id}")
    print(f"  响应: {json.dumps(result, indent=2, ensure_ascii=False)}")


# ============================================================
# 主入口
# ============================================================

def main():
    print("Veo 3.1 视频生成测试脚本")
    print(f"  BASE_URL: {BASE_URL}")
    print(f"  MODEL: {MODEL}")
    print()

    # 如果有命令行参数，作为 task_id 直接查询
    if len(sys.argv) > 1:
        example_query_task(sys.argv[1])
        return

    # 运行所有示例（需要替换为真实图片路径）
    # example_text_to_video()
    # example_image_to_video("first_frame.png")
    example_first_last_frame("first_frame.png", "last_frame.png")
    # example_first_last_frame_data_uri("first_frame.png", "last_frame.png")
    # example_reference_images(["ref1.png", "ref2.png", "ref3.png"])

    print("请取消注释上面的示例函数来运行测试。")
    print()
    print("用法:")
    print("  1. 设置环境变量:")
    print("     export NEWAPI_BASE_URL=https://your-newapi-domain")
    print("     export NEWAPI_API_KEY=sk-xxxx")
    print()
    print("  2. 取消注释 main() 中的示例函数")
    print()
    print("  3. 或直接查询已有任务:")
    print("     python test_generation_video.py task_xxxx")


if __name__ == "__main__":
    main()