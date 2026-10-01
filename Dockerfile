FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
COPY docs/openapi.yaml /src/docs/openapi.yaml
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/migrate ./cmd/migrate

FROM alpine:3.22
ARG APP_VERSION=dev
LABEL org.opencontainers.image.version=$APP_VERSION
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=backend /out/ ./
COPY migrations/ ./migrations/
COPY --from=frontend /src/web/dist/ ./web/dist/
USER app
EXPOSE 8080
CMD ["sh", "-c", "./migrate && exec ./api"]
