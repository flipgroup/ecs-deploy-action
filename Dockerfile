FROM golang:1.15-alpine

COPY . /ecs-deploy-action

WORKDIR /ecs-deploy-action

RUN go install

ENTRYPOINT ["/go/bin/ecs-deploy-action"]
