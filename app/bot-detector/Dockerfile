# ---- build stage --------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/bot-detector .

# ---- runtime stage ------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bot-detector /bot-detector
USER nonroot:nonroot
ENTRYPOINT ["/bot-detector"]
