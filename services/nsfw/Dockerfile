FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY app.py .
# Download the model weights at build time so cold starts don't hit the network.
RUN python -c "import opennsfw2 as n2; n2.make_open_nsfw_model()"
ENV PORT=8000
CMD gunicorn -w 1 --threads 4 -t 60 -b [::]:$PORT app:app
