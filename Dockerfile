FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /firstbyte .

FROM scratch
COPY --from=builder /firstbyte /firstbyte
COPY --from=builder /src/template/email.html /template/email.html
ENTRYPOINT ["/firstbyte"]
