CGO_ENABLED=0 GOOS=linux go build .
rm -rf dist
go run generator.go
docker build -t awencasa/awen .
