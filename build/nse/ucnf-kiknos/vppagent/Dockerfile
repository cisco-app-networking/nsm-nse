ARG VPP_AGENT

FROM golang:alpine as build
RUN apk --no-cache add git
ENV PACKAGEPATH=github.com/networkservicemesh/networkservicemesh/
ENV GO111MODULE=on

RUN mkdir /root/nsm-nse
ADD ["go.mod","/root/nsm-nse"]
WORKDIR /root/nsm-nse/
RUN go mod download

ADD [".","/root/nsm-nse"]
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o /go/bin/cnf-vppagent ./cmd/universal-cnf-vpp/...

FROM ${VPP_AGENT} as runtime
COPY --from=build /go/bin/cnf-vppagent /bin/cnf-vppagent
COPY ./build/nse/universal-cnf/vppagent/etc/ /etc/
COPY ./build/nse/universal-cnf/vppagent/etc/supervisord/supervisord.conf /opt/vpp-agent/dev/supervisor.conf
RUN rm /opt/aio-agent/dev/etcd.conf; \
    echo 'Endpoint: "localhost:9113"' > /opt/aio-agent/dev/grpc.conf ; \
    echo "disabled: true" > /opt/aio-agent/dev/telemetry.conf; \
    echo "disabled: true" > /opt/aio-agent/dev/linux-plugin.conf
