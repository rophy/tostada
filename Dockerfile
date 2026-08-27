FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /tostada ./cmd/tostada/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=backend /tostada /tostada
COPY config.yaml /etc/tostada/config.yaml
ENTRYPOINT ["/tostada", "-config", "/etc/tostada/config.yaml"]
