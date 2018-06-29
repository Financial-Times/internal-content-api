[![Circle CI](https://circleci.com/gh/Financial-Times/internal-content-api.svg?style=shield)](https://circleci.com/gh/Financial-Times/internal-content-api)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/internal-content-api)](https://goreportcard.com/report/github.com/Financial-Times/internal-content-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/internal-content-api/badge.svg?branch=master)](https://coveralls.io/github/Financial-Times/internal-content-api?branch=master)

# Internal Content API (internal-content-api)

__Internal Content API serves published content that includes the internal components beside the enriched content fields__

## Installation

For the first time:

curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
go get -u github.com/Financial-Times/internal-content-api
cd $GOPATH/src/github.com/Financial-Times/internal-content-api
dep ensure
go build .

## Running


## Running locally
_How can I run it_

1. Run the tests and install the binary:

        dep ensure
        go test -race ./...
        go install
2. Run the binary locally with properties set:

```
go install
$GOPATH/bin/internal-content-api \
--app-port  "8084" \
--handler-path "internalcontent" \
--content-source-uri "http://localhost:8080/__enriched-content-read-api/enrichedcontent \
--internal-components-source-uri "http://localhost:8080/__document-store-api/internalcomponents/" \
--content-source-app-name  "Content Source Service" \
--internal-components-source-app-name  "Internal Components Source Service" \
--content-source-app-health-uri  "http://localhost:8080/__enriched-content-read-api/__health" \
--internal-components-source-app-health-uri  "http://localhost:8080/__document-store-api/__health" \
--content-unroller-app-name "Content unroller" \
--content-unroller-uri "http://localhost:8080/__content-unroller-api/internalcontent" \
--content-unroller-app-health-uri "http://localhost:8080/__content-unroller-api/__health" \
--graphite-tcp-address "graphite.ft.com:2003" \
--graphite-prefix "coco.services.$ENV.content-preview.%i"
 
```

With Docker:

`docker build -t coco/internal-content-api .`

`docker run -ti coco/internal-content-api`

```
docker run -ti  
--env "APP_PORT=8080" \
--env "HANDLER_PATH=internalcontent" \
--env "CONTENT_SOURCE_URI=http://localhost:8080/__enriched-content-read-api/enrichedcontent/" \
--env "INTERNAL_COMPONENTS_SOURCE_URI=http://localhost:8080/__document-store-api/internalcomponents/" \
--env "CONTENT_SOURCE_APP_NAME=Content Source Service" \
--env "INTERNAL_COMPONENTS_SOURCE_APP_NAME=Document Store Service" \
--env "CONTENT_SOURCE_APP_HEALTH_URI=http://localhost:8080/__enriched-content-read-api/__health" \
--env "INTERNAL_COMPONENTS_SOURCE_APP_HEALTH_URI=http://localhost:8080/__document-store-api/__health" \
--env "CONTENT_UNROLLER_URI=http://localhost:8080/__content-unroller-api/internalcontent"
--env "CONTENT_UNROLLER_APP_NAME=content-unroller"
--env "CONTENT_UNROLLER_APP_HEALTH_URI=http://localhost:8080/__content-unroller-api/__health"
--env "GRAPHITE_TCP_ADDRESS=graphite.ft.com:2003" \  
--env "GRAPHITE_PREFIX=coco.services.$ENV.content-preview.%i" \  
coco/internal-content-api  
```

When deployed locally arguments are optional.

## Endpoints
### GET
/internalcontent/{uuid}    
Example
`curl -v http://localhost:8084/internalcontent/9358ba1e-c07f-11e5-846f-79b0e3d20eaf`

The read should return the internal content of an article (i.e. an aggregation of enriched content plus internal components).

#### Optional Parameters
`unrollContent={boolean}`, default *false*

When `true` all the images in the response(main image, body embedded images, lead images and alternative images) get expanded with the content as content-public-read service was called for that image. This service does not call content-public-read directly, but uses image-resolver which is responsible to get the requested images. When `false` the response contains only the IDs to the images.

404 if article with given uuid does not exist.

503 when one of the collaborating mandatory services is inaccessible.

In case `handler-path` / `HANDLER_PATH` is set to something else other than `internalcontent`,
for example to `internalcontent-preview`, the endpoint will change accordingly to:

/internalcontent-preview/{uuid}

Example in this case will be:
`curl -v http://localhost:8084/internalcontent-preview/9358ba1e-c07f-11e5-846f-79b0e3d20eaf`

### Admin endpoints
Healthchecks: [http://localhost:8084/__health](http://localhost:8084/__health)

good-to-go: [http://localhost:8084/__gtg](http://localhost:8084/__gtg)

Health and gtg are based on enriched-content-read-api and document-store-api's health endpoints availability.

Ping: [http://localhost:8084/__ping](http://localhost:8084/__ping)

Build-info: [http://localhost:8084/__build-info](http://localhost:8084/__build-info)  -  [Documentation on how to generate build-info] (https://github.com/Financial-Times/service-status-go) 
 
Metrics:  [http://localhost:8084/__metrics](http://localhost:8084/__metrics)

## Model

For the model spec please refer to:
* [enriched-content-read-api](http://git.svc.ft.com/projects/CP/repos/enriched-content-read-api/browse) - complements the content model with content & concept relationships
* [methode-article-internal-components-mapper](https://github.com/Financial-Times/methode-article-internal-components-mapper) - transforms the internal components of a story (this part of the model is subject to continuous change, hence for the latest model it's recommended to check the [spec](https://github.com/Financial-Times/methode-article-internal-components-mapper/blob/master/api.md))
