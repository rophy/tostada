ARG DEVELOPMENT=0

FROM node:22-alpine AS frontend
ARG DEVELOPMENT
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN DEVELOPMENT=$DEVELOPMENT npm run build

FROM golang:1.22-alpine AS backend
ARG DEVELOPMENT
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN if [ "$DEVELOPMENT" = "1" ]; then \
      CGO_ENABLED=0 go build -cover -covermode=atomic -tags coverage -o /tostada ./cmd/tostada/; \
    else \
      CGO_ENABLED=0 go build -o /tostada ./cmd/tostada/; \
    fi
RUN CGO_ENABLED=0 go build -o /tostada-cli ./cmd/tostada-cli/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
ENV TOSTADA_DB=/data/tostada.db
COPY --from=backend /tostada /tostada
COPY --from=backend /tostada-cli /tostada-cli
ENTRYPOINT ["/tostada", "-config", "/etc/tostada/config.yaml"]
