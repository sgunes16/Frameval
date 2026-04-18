FROM golang:1.25-bookworm AS engine-builder
WORKDIR /app
COPY engine/go.mod engine/go.sum* ./engine/
RUN cd engine && go mod download
COPY engine ./engine
COPY proto ./proto
RUN cd engine && CGO_ENABLED=1 GOOS=linux go build -o /out/frameval-engine ./cmd/server

FROM node:20-bookworm AS frontend-dev
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend ./
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]

FROM node:20-bookworm-slim AS engine-runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates sqlite3 bash curl git python3 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=engine-builder /out/frameval-engine /usr/local/bin/frameval-engine
COPY tasks ./tasks
COPY baselines ./baselines
EXPOSE 8080
CMD ["frameval-engine"]
