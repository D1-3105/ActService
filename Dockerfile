# tbd
FROM golang:1.24.4-bookworm AS builder

COPY . /actor

WORKDIR /actor
RUN go mod tidy

RUN make compile_grpc
RUN make install_act_persistent

RUN go build -o /bin/actor main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /actor/actor /actor
COPY --from=builder /bin/act /bin/act
ENTRYPOINT ["/actor"]
