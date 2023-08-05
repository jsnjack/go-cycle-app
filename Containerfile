FROM golang:1.20 AS builder

WORKDIR /app
COPY . .

# Install all dependencies
RUN go mod download

ADD https://github.com/jsnjack/grm/releases/download/v0.54.0/grm /usr/local/bin/grm
RUN chmod +x /usr/local/bin/grm

# Automatically calculate version
RUN /usr/local/bin/grm install monova -y

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-X github.com/jsnjack/go-cycle-app/cmd.Version=${VERSION}" -o go-cycle-app .

FROM fedora:38

WORKDIR /app

COPY --from=builder /app/go-cycle-app .

ENTRYPOINT ["/bin/bash", "-c", "./go-cycle-app -p $PORT -d $DOMAIN -f $DB_FILE -i $STRAVA_APP_ID -s $STRAVA_APP_SECRET"]
