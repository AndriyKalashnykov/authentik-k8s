# syntax=docker/dockerfile:1

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
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/authentik-poc .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/authentik-poc /usr/local/bin/authentik-poc
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/authentik-poc"]
