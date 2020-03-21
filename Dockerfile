FROM alpine:3.11

RUN addgroup -S prefixrouter \
    && adduser -S -g prefixrouter prefixrouter \
    && apk --no-cache add ca-certificates

WORKDIR /home/prefixrouter

COPY /bin/prefixrouter .

RUN chown -R prefixrouter:prefixrouter ./

USER prefixrouter

ENTRYPOINT ["./prefixrouter"]

