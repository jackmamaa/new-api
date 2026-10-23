import base64, requests

with open("start-frame.jpg", "rb") as f:
    first_b64 = base64.b64encode(f.read()).decode()
with open("end-frame.jpg", "rb") as f:
    last_b64 = base64.b64encode(f.read()).decode()

url = "https://new.sysrv.net/v1/video/generations"
headers = {
    "Authorization": "Bearer sk-MQC98iHFxO0ZxGtLFaraSemawXbttnCKd1BWsaQT8PS6Rl1g",
    "Content-Type": "application/json"
}
data = {
    "model": "veo-3.1-fast-generate-preview",
    "prompt": "首帧平视推进到店铺内，店员哼着bigbang的小曲，和尾帧无缝衔接",
    "metadata": {
        "image": {
            "inlineData": {
                "mimeType": "image/jpeg",
                "data": first_b64
            }
        },
        "lastFrame": {
            "inlineData": {
                "mimeType": "image/jpeg",
                "data": last_b64
            }
        },
        "aspectRatio": "9:16",
        "durationSeconds": 4,
        "generateAudio": True,
        "responseFormat": "privacy_url"
    }
}
resp = requests.post(url, headers=headers, json=data)
print(resp.json())