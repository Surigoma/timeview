# syntax=docker/dockerfile:1

FROM node:24-alpine AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci
COPY web ./web
RUN npm --prefix web run build

FROM golang:1.27-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -tags=nomsgpack -buildvcs=false -trimpath -ldflags="-s -w" -o /timeview .
RUN mkdir /data && chown 65532:65532 /data

FROM scratch
COPY --from=go-build /timeview /timeview
COPY --from=go-build --chown=65532:65532 /data /data
USER 65532:65532
WORKDIR /data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/timeview"]
CMD ["-listen", "0.0.0.0:8080", "-config", "/data/timeview-config.json", "-audit-log", "/data/timeview-operations.jsonl"]
