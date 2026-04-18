FROM python:3.11-slim-bookworm
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
COPY grader ./grader
COPY proto ./proto
RUN pip install --no-cache-dir uv && uv pip install --system ./grader
EXPOSE 50051
CMD ["python", "-m", "grader.server"]
