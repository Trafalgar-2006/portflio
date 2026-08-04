FROM golang:latest AS builder

ENV GOTOOLCHAIN=auto
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG BUILD_COMMIT=unknown
# Default to image build time so the /now freshness stamp is right even when
# the platform doesn't pass --build-arg BUILD_DATE.
ARG BUILD_DATE=""
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.BuildCommit=${BUILD_COMMIT} -X main.BuildDate=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" \
    -o ssh-portfolio .

FROM alpine:latest

RUN apk --no-cache add ca-certificates openssh-keygen

WORKDIR /app
COPY --from=builder /app/ssh-portfolio .
COPY --from=builder /app/content.yaml .
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

ENV SSH_ENABLED=true
ENV COLORTERM=truecolor
ENV TERM=xterm-256color
EXPOSE 23234
EXPOSE 8080

CMD ["./entrypoint.sh"]
