# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89

# Build a static binary, ship it on distroless-nonroot. This is a one-shot job
# (creates groups/users/tokens then exits), so there is no EXPOSE/HEALTHCHECK —
# all config is supplied at runtime via env (see .env.example / `make image-run`).

# GO_VERSION must match go.mod + .mise.toml (enforced by `make check-toolchain-alignment`).
ARG GO_VERSION=1.26.4

FROM golang:${GO_VERSION} AS build
WORKDIR /src
# Cache deps separately from source.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO disabled → fully static binary for the distroless/static base.
# Dependencies are pure Go (goauthentik client + go-openapi), so this is safe.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/provisioner .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639
COPY --from=build /out/provisioner /usr/local/bin/provisioner
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/provisioner"]
