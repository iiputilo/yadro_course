import os
import time
import io
import asyncio
import logging
import base64
import requests
from requests.exceptions import ReadTimeout, RequestException
from telegram import Update
from telegram.ext import Application, CommandHandler, ContextTypes

import httpx

BOT_TOKEN = os.getenv("BOT_TOKEN", "")
API_BASE_URL = os.getenv("API_BASE_URL", "http://api:8080").rstrip("/")
ADMIN_USER = os.getenv("ADMIN_USER", "admin")
ADMIN_PASSWORD = os.getenv("ADMIN_PASSWORD", "password")

DB_UPDATE_POST_TIMEOUT_SEC = int(os.getenv("DB_UPDATE_POST_TIMEOUT_SEC", "60"))
DB_UPDATE_WAIT_TIMEOUT_SEC = int(os.getenv("DB_UPDATE_WAIT_TIMEOUT_SEC", "300"))
DB_UPDATE_POLL_INTERVAL_SEC = float(os.getenv("DB_UPDATE_POLL_INTERVAL_SEC", "2"))

OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY", "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").strip()
OPENROUTER_MODEL = os.getenv("OPENROUTER_MODEL", "openai/gpt-5").strip()
OPENROUTER_API_URL = os.getenv("OPENROUTER_API_URL", "https://openrouter.ai/api/v1/chat/completions").strip()

OPENROUTER_CONNECT_TIMEOUT = float(os.getenv("OPENROUTER_CONNECT_TIMEOUT", "30"))
OPENROUTER_READ_TIMEOUT = float(os.getenv("OPENROUTER_READ_TIMEOUT", "300"))
OPENROUTER_WRITE_TIMEOUT = float(os.getenv("OPENROUTER_WRITE_TIMEOUT", "60"))
OPENROUTER_POOL_TIMEOUT = float(os.getenv("OPENROUTER_POOL_TIMEOUT", "60"))

OPENROUTER_TIMEOUT = httpx.Timeout(
    connect=OPENROUTER_CONNECT_TIMEOUT,
    read=OPENROUTER_READ_TIMEOUT,
    write=OPENROUTER_WRITE_TIMEOUT,
    pool=OPENROUTER_POOL_TIMEOUT,
)

OPENROUTER_REQUEST_TIMEOUT_SEC = float(os.getenv("OPENROUTER_REQUEST_TIMEOUT_SEC", "120"))

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def api_get(
    path: str,
    params: dict | None = None,
    headers: dict | None = None,
    timeout: int = 30,
) -> requests.Response:
    return requests.get(f"{API_BASE_URL}{path}", params=params, headers=headers, timeout=timeout)


def api_post(
    path: str,
    json_body: dict | None = None,
    headers: dict | None = None,
    timeout: int = 30,
) -> requests.Response:
    return requests.post(f"{API_BASE_URL}{path}", json=json_body, headers=headers, timeout=timeout)


def safe_json(resp: requests.Response) -> dict:
    try:
        return resp.json()
    except Exception:
        return {}


async def cmd_ping(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    try:
        resp = await asyncio.to_thread(api_get, "/api/ping", None, None, 15)
    except RequestException as e:
        await update.message.reply_text(f"ping failed: {e}")
        return

    if not resp.ok:
        await update.message.reply_text(f"ping failed: {resp.status_code} {resp.text}")
        return

    await update.message.reply_text(resp.text)


def api_login_token() -> str:
    resp = api_post("/api/login", json_body={"name": ADMIN_USER, "password": ADMIN_PASSWORD}, timeout=15)
    if not resp.ok:
        raise RuntimeError(f"login failed: {resp.status_code} {resp.text}")
    return resp.text.strip()


def wait_update_idle(headers: dict, total_timeout_sec: int) -> tuple[bool, str]:
    deadline = time.time() + total_timeout_sec
    last_status = "unknown"

    while time.time() < deadline:
        try:
            st = api_get("/api/db/status", headers=headers, timeout=15)
        except RequestException as e:
            last_status = f"status_request_failed: {e}"
            time.sleep(DB_UPDATE_POLL_INTERVAL_SEC)
            continue

        if not st.ok:
            last_status = f"http_{st.status_code}: {st.text}"
            time.sleep(DB_UPDATE_POLL_INTERVAL_SEC)
            continue

        data = safe_json(st)
        last_status = data.get("status", "unknown")

        if last_status == "idle":
            return True, last_status

        time.sleep(DB_UPDATE_POLL_INTERVAL_SEC)

    return False, last_status


async def cmd_update_db(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    try:
        token = await asyncio.to_thread(api_login_token)
    except Exception as e:
        await update.message.reply_text(f"auth error: {e}")
        return

    headers = {"Authorization": f"Token {token}"}

    try:
        resp = await asyncio.to_thread(api_post, "/api/db/update", None, headers, DB_UPDATE_POST_TIMEOUT_SEC)
    except ReadTimeout:
        await update.message.reply_text("update request timeout; checking status...")
        resp = None
    except RequestException as e:
        await update.message.reply_text(f"update request failed: {e}")
        return

    if resp is not None:
        body = safe_json(resp)
        status = body.get("status")

        if resp.status_code == 401:
            await update.message.reply_text(f"unauthorized (401): {resp.text}")
            return
        if resp.status_code not in (200, 202):
            await update.message.reply_text(f"update failed: {resp.status_code} {resp.text}")
            return

        if status == "already_running":
            await update.message.reply_text("update already running; waiting for completion...")
        else:
            await update.message.reply_text("update started; waiting for completion...")

    ok, last = await asyncio.to_thread(wait_update_idle, headers, DB_UPDATE_WAIT_TIMEOUT_SEC)
    if not ok:
        await update.message.reply_text(
            f"update not finished within {DB_UPDATE_WAIT_TIMEOUT_SEC}s; last status: {last}"
        )
        return

    try:
        st = await asyncio.to_thread(api_get, "/api/db/stats", None, headers, 15)
    except RequestException as e:
        await update.message.reply_text(f"update done, but stats request failed: {e}")
        return

    if not st.ok:
        await update.message.reply_text(f"update done, but stats failed: {st.status_code} {st.text}")
        return

    await update.message.reply_text(f"update done; stats: {st.text}")


def _openrouter_headers(api_key: str) -> dict:
    return {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
        "Accept": "application/json",
        "HTTP-Referer": "https://github.com/iiputilo/openrouter_tg_bot",
        "X-Title": "openrouter_tg_bot",
    }


def _img_bytes_to_data_uri(img: bytes, mime: str) -> str:
    b64 = base64.b64encode(img).decode("ascii")
    return f"data:{mime};base64,{b64}"


def _extract_text_from_chat_completions(data: dict) -> str:
    choices = data.get("choices")
    if not isinstance(choices, list) or not choices:
        return ""
    raw = (choices[0] or {}).get("message", {}).get("content", "")
    if isinstance(raw, list):
        parts: list[str] = []
        for part in raw:
            if isinstance(part, dict) and part.get("text"):
                parts.append(str(part.get("text", "")))
        return "\n".join(parts).strip()
    return str(raw or "").strip()


async def _explain_comic_via_openrouter(image_bytes: bytes, mime: str) -> str:
    if not OPENROUTER_API_KEY:
        return "Пояснение недоступно: не задан `OPENROUTER_API_KEY`."

    data_uri = _img_bytes_to_data_uri(image_bytes, mime)

    payload = {
        "model": OPENROUTER_MODEL,
        "messages": [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": "объясни смысл этого комикса"},
                    {"type": "image_url", "image_url": {"url": data_uri}},
                ],
            }
        ],
    }

    async def _do() -> str:
        async with httpx.AsyncClient(timeout=OPENROUTER_TIMEOUT) as client:
            resp = await client.post(
                OPENROUTER_API_URL,
                json=payload,
                headers=_openrouter_headers(OPENROUTER_API_KEY),
            )

        if resp.status_code >= 400:
            try:
                err_json = resp.json()
                msg = (
                    (err_json.get("error") or {}).get("message")
                    or err_json.get("message")
                    or resp.text
                )
            except Exception:
                msg = resp.text
            return f"OpenRouter error {resp.status_code}: {str(msg).strip()}"

        try:
            data = resp.json()
        except Exception:
            return "OpenRouter вернул неожиданный ответ. Попробуйте позже."

        text = _extract_text_from_chat_completions(data)
        return text if text else "OpenRouter вернул пустое пояснение."

    try:
        return await asyncio.wait_for(_do(), timeout=OPENROUTER_REQUEST_TIMEOUT_SEC)
    except asyncio.TimeoutError:
        return "OpenRouter timeout. Попробуйте позже."
    except httpx.HTTPError as e:
        return f"OpenRouter network error: {e}"
    except Exception as e:
        logger.exception("openrouter explain failed")
        return f"OpenRouter error: {e}"


async def cmd_search(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    if update.message is None:
        return

    if not context.args:
        await update.message.reply_text("usage: /search <phrase>")
        return

    phrase = " ".join(context.args).strip()
    if not phrase:
        await update.message.reply_text("usage: /search <phrase>")
        return

    try:
        resp = await asyncio.to_thread(api_get, "/api/isearch", {"phrase": phrase, "limit": "1"}, None, 30)
    except RequestException as e:
        await update.message.reply_text(f"search failed: {e}")
        return

    if not resp.ok:
        await update.message.reply_text(f"search failed: {resp.status_code} {resp.text}")
        return

    data = safe_json(resp)
    comics = data.get("comics") or []
    if not comics:
        await update.message.reply_text("no results")
        return

    url = (comics[0] or {}).get("url")
    cid = (comics[0] or {}).get("id")
    if not url:
        await update.message.reply_text("search result has no url")
        return

    try:
        img_resp = await asyncio.to_thread(requests.get, str(url), timeout=30)
        img_resp.raise_for_status()
    except RequestException as e:
        await update.message.reply_text(f"failed to download image: {e}")
        return

    content_type = (img_resp.headers.get("Content-Type") or "").lower().split(";")[0].strip()
    if not content_type or "image/" not in content_type:
        await update.message.reply_text(f"downloaded content is not an image: {content_type}")
        return

    processing = await update.message.reply_text("Обрабатываю комикс и запрашиваю пояснение…")

    explanation = await _explain_comic_via_openrouter(img_resp.content, content_type)

    try:
        await processing.delete()
    except Exception:
        pass

    caption_lines: list[str] = []
    if cid is not None:
        caption_lines.append(f"id: {cid}")
    caption_lines.append(str(url))
    caption_lines.append("")
    caption_lines.append("Пояснение:")
    caption_lines.append(explanation)
    caption = "\n".join(caption_lines).strip()

    bio = io.BytesIO(img_resp.content)
    bio.name = "comic"
    await update.message.reply_photo(photo=bio, caption=caption)


async def on_error(update: object, context: ContextTypes.DEFAULT_TYPE) -> None:
    logger.exception("Unhandled error while processing update", exc_info=context.error)


def main() -> None:
    if not BOT_TOKEN:
        raise RuntimeError("BOT_TOKEN env is required")

    app = Application.builder().token(BOT_TOKEN).build()
    app.add_handler(CommandHandler("ping", cmd_ping))
    app.add_handler(CommandHandler("search", cmd_search))
    app.add_handler(CommandHandler("update_db", cmd_update_db))
    app.add_error_handler(on_error)
    app.run_polling(close_loop=False)


if __name__ == "__main__":
    main()
