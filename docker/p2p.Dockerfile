FROM golang as build
WORKDIR /app
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./p2p

FROM debian
COPY --from=build /app/server /app/server
CMD "/app/server"
