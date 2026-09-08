# syntax=docker/dockerfile:1
FROM python:3.11-slim

WORKDIR /app

# Install dependencies first — Docker caches this layer separately from source
# code, so a code-only change won't trigger a full pip install.
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application source
COPY . .

EXPOSE 8000

# Default command runs the FastAPI service.
# The celery-worker service in docker-compose overrides this CMD.
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000", "--reload"]
