FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/anzhen ./cmd/server

FROM alpine:3.21
RUN addgroup -S anzhen && adduser -S anzhen -G anzhen
WORKDIR /app
COPY --from=build /out/anzhen /app/anzhen
COPY --from=build /src/web /app/web
RUN mkdir -p /app/data && chown -R anzhen:anzhen /app
USER anzhen
ENV PORT=8080 DATA_DIR=/app/data APP_ENV=production
EXPOSE 8080
CMD ["/app/anzhen"]
