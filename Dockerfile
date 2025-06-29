# tbd
FROM golang:1.24.4-bookworm AS builder

COPY . /actor

WORKDIR /actor
RUN go mod tidy

RUN make install_act_persistent

RUN go build -o /bin/actor main.go

FROM debian:bookworm-slim
# install docker
RUN apt-get update && \
    apt-get install -y docker.io && \
    apt-get clean && rm -rf /var/lib/apt/lists/*
COPY --from=builder /bin/actor /actor
COPY --from=builder /bin/act /bin/act
ENTRYPOINT ["/actor"]
