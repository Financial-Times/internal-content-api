FROM golang:1

ENV PROJECT=internal-content-api

COPY . /${PROJECT}/
WORKDIR /${PROJECT}

RUN BUILDINFO_PACKAGE="github.com/Financial-Times/service-status-go/buildinfo." \
    && VERSION="version=$(git describe --tag --always 2> /dev/null)" \
    && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
    && REPOSITORY="repository=$(git config --get remote.origin.url)" \
    && REVISION="revision=$(git rev-parse HEAD)" \
    && BUILDER="builder=$(go version)" \
    && LDFLAGS="-s -w -X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
    && CGO_ENABLED=0 go build -mod=readonly -a -o /artifacts/${PROJECT} -ldflags="${LDFLAGS}"

# Multi-stage build - copy only the certs and the binary into the image
FROM scratch
WORKDIR /
COPY ./api/api.yml /
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /artifacts/* /

CMD ["/internal-content-api" ]
