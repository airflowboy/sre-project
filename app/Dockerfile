# ---- build stage --------------------------------------------------------
# Full Go toolchain — only used to compile; not shipped in the final image.
FROM golang:1.25-alpine AS build
WORKDIR /src

# No third-party deps yet, so copying go.mod alone is enough to cache the
# (empty) module graph. Add go.sum here too once real dependencies appear.
COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
# CGO disabled → fully static binary, runnable on a scratch/distroless base.
# -trimpath + -s -w shrink the binary and strip absolute paths.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/web-app .

# ---- runtime stage ------------------------------------------------------
# distroless/static: no shell, no package manager — tiny attack surface.
# Use the :debug variant locally if you need to `kubectl exec` a shell.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/web-app /web-app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/web-app"]
