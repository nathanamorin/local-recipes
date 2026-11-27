# Multistage build: Go builds the API server and MCP server binaries,
# Node builds the React SPA, and the final image is a minimal Alpine
# runtime with both Go binaries plus the built frontend's static assets.
# Unlike local-podcasts, nothing here dlopens a glibc-pinned native
# library, so a lightweight Alpine base works fine for both stages.
FROM golang:1.26-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o local-recipes ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o local-recipes-mcpserver ./cmd/mcpserver

FROM node:20-alpine AS frontend

WORKDIR /app
COPY ./app/local-recipes/ ./
RUN npm install
RUN npm run build

##
## Deploy
##
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=build /build/local-recipes /app/local-recipes
COPY --from=build /build/local-recipes-mcpserver /app/local-recipes-mcpserver
COPY --from=frontend /app/build /app/static/

RUN chown -R app:app /app
USER app:app

# Documentation only - actual port exposure is configured at the
# Service/Deployment level. 8080 is the API+SPA server, 8090 the MCP
# server (see cmd/server and cmd/mcpserver's appConfig defaults).
EXPOSE 8080 8090

CMD ["/app/local-recipes"]
