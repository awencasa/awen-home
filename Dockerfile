FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY ./awen /app/awen
COPY ./dist /app/dist
CMD ["/app/awen"]
