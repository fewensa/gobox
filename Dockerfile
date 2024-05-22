FROM node:16-alpine as builder
RUN mkdir -p /opt/build
WORKDIR /opt/build
COPY ./relayer ./
RUN yarn install && yarn build

# build golang
FROM golang:1.20-alpine as go-builder
WORKDIR /build
COPY ./go.mod .
RUN go mod download
COPY . .
RUN go build -o ./gbox .

FROM node:16-alpine
RUN apk update && apk add expect curl
RUN mkdir -p /opt/data
COPY --from=builder /opt/build/dist /opt/relayer/dist
COPY --from=go-builder /build/gbox /opt/relayer/runner
WORKDIR /opt/relayer
COPY ./relayer/.env.docker .env
COPY ./relayer/package.json package.json
RUN yarn install --production && mkdir -p ./.maintain/db
ENV CONFIG_PATH=./.maintain/configure.json
ENV HELIX_ROOT_DIR=/opt/relayer
ENV HELIX_ENV="LP_BRIDGE_PATH=./.maintain/configure.json,LP_BRIDGE_STORE_PATH=./.maintain/db"
ENV HELIX_COMMAND="node ./dist/src/main"
ENTRYPOINT ["/opt/relayer/runner"]

