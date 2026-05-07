### Stage 1: Build the Vue frontend
### npm install is cached until package*.json changes — Go source changes don't bust this layer.
FROM node:22 AS web
WORKDIR /web
COPY server/app/package*.json ./
RUN npm ci
COPY server/app/ ./
RUN npm run build

### Stage 2: Build the Go binary
### go mod download is cached until go.mod/go.sum change — source changes don't re-download deps.
FROM golang AS build
WORKDIR /go/src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /web/dist ./server/app/dist

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown
ENV CGO_ENABLED=0
RUN go build \
    -ldflags "-X main.version=${VERSION} -X main.commitSHA=${COMMIT_SHA} -X main.buildDate=${BUILD_DATE}" \
    -o openbooks \
    ./cmd/openbooks

### Stage 3: Minimal distroless runtime — just copy the binary
FROM gcr.io/distroless/static AS app
WORKDIR /app
COPY --from=build /go/src/openbooks .

EXPOSE 80
VOLUME ["/books"]
ENV BASE_PATH=/

ENTRYPOINT ["./openbooks"]
CMD ["server", "--name", "openbooks_abs", "--dir", "/books", "--port", "80"]
