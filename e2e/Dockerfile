FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test -tags e2e -c -o /e2e-api ./e2e/api/

FROM mcr.microsoft.com/playwright:v1.52.0-noble
WORKDIR /e2e

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /e2e-api /usr/local/bin/e2e-api

COPY e2e/web/package.json e2e/web/package-lock.json ./web/
RUN cd web && npm ci

COPY e2e/web/ ./web/

COPY e2e/run.sh /run.sh
RUN chmod +x /run.sh

ENV TOSTADA_E2E_URL=http://gateway:8080
ENTRYPOINT ["/run.sh"]
