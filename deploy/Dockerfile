# Multi-stage build: a single static binary in a distroless image (SYSTEM_DESIGN §13),
# with the built PWA beside it, served by API mode from FUXI_WEB_DIR (§10).
# Build from the repository root:  docker build -f deploy/Dockerfile .

FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build && npm run size

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/fuxi ./cmd/fuxi

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/fuxi /fuxi
COPY --from=web /web/dist /web
ENV FUXI_WEB_DIR=/web
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/fuxi"]
CMD ["api"]
