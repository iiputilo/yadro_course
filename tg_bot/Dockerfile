FROM python:3.12-slim

WORKDIR /app

COPY tg_bot/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

COPY tg_bot /app/tg_bot

ENV PYTHONUNBUFFERED=1

CMD ["python", "-m", "tg_bot.bot"]
