FROM registry.access.redhat.com/ubi9/ubi-minimal:9.2-750.1697625013

LABEL maintainer="Tom Manville<tom@kasten.io>"

RUN microdnf -y update openssl-libs

ADD controller /controller
ENTRYPOINT ["/controller"]
