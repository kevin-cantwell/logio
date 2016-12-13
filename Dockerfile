FROM timehop/alpine:latest

# Before building this container run:
# GOOS=linux GOARCH=amd64 go build -o bin/logio-server cmd/logio-server/main.go
ADD bin/logio-server /usr/bin/logio-server

EXPOSE 7701
EXPOSE 7702
CMD /usr/bin/logio-server