FROM golang:1.22.12-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY migrations ./migrations
ARG SERVICE=api
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /out/service ./cmd/${SERVICE} \
 && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /out/healthcheck ./cmd/healthcheck

FROM scratch AS runtime
WORKDIR /app
COPY --from=build /src/migrations/ /app/migrations/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/ /app/
USER 65532:65532
CMD ["/app/service"]
