# syntax=docker/dockerfile:1

FROM node:26.13.1-alpine3.24@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS web-build
# npm 12.2.1 still bundles vulnerable undici/ip-address versions. Patch only
# those build-time packages until npm ships their fixes (CVE-2026-109869,
# CVE-2026-109924 and CVE-2026-111997).
RUN npm install --global --ignore-scripts --no-audit --no-fund npm@12.2.1 \
    && npm install --prefix /tmp/npm-security --ignore-scripts --no-save --package-lock=false --no-audit --no-fund undici@6.29.1 ip-address@10.7.4 \
    && rm -rf /usr/local/lib/node_modules/npm/node_modules/undici /usr/local/lib/node_modules/npm/node_modules/ip-address \
    && mv /tmp/npm-security/node_modules/undici /usr/local/lib/node_modules/npm/node_modules/undici \
    && mv /tmp/npm-security/node_modules/ip-address /usr/local/lib/node_modules/npm/node_modules/ip-address \
    && rm -rf /tmp/npm-security /root/.npm
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci
COPY web ./web
RUN npm --prefix web run build

FROM golang:1.27.4-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS go-build
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
