FROM golang:1.26.8-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
	&& CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /out/api /api
COPY --from=builder /out/migrate /migrate
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/api", "healthcheck"]
ENTRYPOINT ["/api"]
