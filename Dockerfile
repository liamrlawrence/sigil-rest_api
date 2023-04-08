FROM golang:1.19-bullseye
LABEL description="Golang API Server"
ARG WORK_DIR=/app


# Setup workdir
WORKDIR $WORK_DIR
COPY . .

# Pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
RUN go mod download && go mod verify

# Compile app
RUN go build -v -o /usr/local/bin/app ${WORK_DIR}/cmd/rest_api

# Open ports
EXPOSE 8000

# Start API server
CMD ["app"]
