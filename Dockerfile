FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/http-server ./cmd && CGO_ENABLED=0 go build -trimpath -o /out/seed ./cmd/seed

# Configuration files are embedded in the binary (assets/*-config.json); secrets come from the environment.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/http-server /out/seed /
USER nonroot
ENV ACTIVE_ENV=PROD
EXPOSE 3000
ENTRYPOINT ["/http-server"]
