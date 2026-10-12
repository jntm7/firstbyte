FROM golang:1.26-alpine AS builder
# CA bundle so the scratch image can verify TLS (HTTPS feeds, SMTP STARTTLS)
RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /firstbyte .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /firstbyte /firstbyte
COPY --from=builder /src/template/email.html /template/email.html
ENTRYPOINT ["/firstbyte"]
