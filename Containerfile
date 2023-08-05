FROM golang:1.20 AS builder

WORKDIR /app
COPY . .

# Install all dependencies
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o go-cycle-app .

FROM fedora:38

WORKDIR /app

COPY --from=builder /app/go-cycle-app .

ENTRYPOINT ["/bin/bash", "-c", "./go-cycle-app -p $PORT -d $DOMAIN -f $DB_FILE -i $STRAVA_APP_ID -s $STRAVA_APP_SECRET -t VERIFY_TOKEN"]
