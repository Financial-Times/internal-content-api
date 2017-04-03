[![Circle CI](https://circleci.com/gh/Financial-Times/internal-content-api.svg?style=shield)](https://circleci.com/gh/Financial-Times/internal-content-api)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/internal-content-api)](https://goreportcard.com/report/github.com/Financial-Times/internal-content-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/internal-content-api/badge.svg?branch=master)](https://coveralls.io/github/Financial-Times/internal-content-api?branch=master)

# Internal Content API (internal-content-api)

__Internal Content API serves published content that includes the internal components beside the enriched content fields__

## Installation

For the first time:

`go get github.com/Financial-Times/internal-content-api`

or update:

`go get -u github.com/Financial-Times/internal-content-api`

## Running


Locally with default configuration:

```
go install
$GOPATH/bin/internal-content-api
```

Locally with properties set:

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

404 if article with given uuid does not exist.

503 when one of the collaborating mandatory services is inaccessible.

In case `handler-path` / `HANDLER_PATH` is set to something else other than `internalcontent`,
for example to `internalcontent-preview`, the endpoint will change accordingly to:

/internalcontent-preview/{uuid}

Example in this case will be:
`curl -v http://localhost:8084/internalcontent-preview/9358ba1e-c07f-11e5-846f-79b0e3d20eaf`

### Admin endpoints
Healthchecks: [http://localhost:8084/__health](http://localhost:8084/__health)

Ping: [http://localhost:8084/__ping](http://localhost:8084/__ping)

Build-info: [http://localhost:8084/__build-info](http://localhost:8084/__build-info)  -  [Documentation on how to generate build-info] (https://github.com/Financial-Times/service-status-go) 
 
Metrics:  [http://localhost:8084/__metrics](http://localhost:8084/__metrics)

## Model

For the model spec please refer to:
* [enriched-content-read-api](http://git.svc.ft.com/projects/CP/repos/enriched-content-read-api/browse) - complements the content model with content & concept relationships
* [methode-article-internal-components-mapper](https://github.com/Financial-Times/methode-article-internal-components-mapper) - transforms the internal components of a story (this part of the model is subject to continuous change, hence for the latest model it's recommended to check the [spec](https://github.com/Financial-Times/methode-article-internal-components-mapper/blob/master/api.md))
