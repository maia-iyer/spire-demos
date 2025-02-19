FROM ghcr.io/spiffe/spiffe-helper:0.8.0 as spiffe-helper

FROM alpine:latest as alpine

RUN apk --no-cache add curl

COPY --from=spiffe-helper /spiffe-helper /spiffe-helper
COPY helper.conf helper.conf

ENTRYPOINT ["/spiffe-helper"]

CMD []
