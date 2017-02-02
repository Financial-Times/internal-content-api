[![Circle CI](https://circleci.com/gh/Financial-Times/internal-content-api.svg?style=shield)](https://circleci.com/gh/Financial-Times/internal-content-api)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/internal-content-api)](https://goreportcard.com/report/github.com/Financial-Times/internal-content-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/internal-content-api/badge.svg)](https://coveralls.io/github/Financial-Times/internal-content-api)

# Internal Content API (internal-content-api)

__Internal Content API serves published content that includes the internal component besides the normal enriched content__

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
--enriched-content-api-uri "http://localhost:8080/__enriched-content-read-api/enrichedconten \
--document-store-api-uri "http://localhost:8080/__document-store-api/internalcomponents/" \
--enriched-content-app-name  "Enriched Content Service" \
--document-store-app-name  "Document Store Service" \
--enriched-content-app-health-uri  "http://localhost:8080/__enriched-content-read-api/__health" \
--document-store-app-health-uri  "http://localhost:8080/__enriched-content-read-api/__health" \
--graphite-tcp-address "graphite.ft.com:2003" \
--graphite-prefix "coco.services.$ENV.content-preview.%i"
 
```

With Docker:

`docker build -t coco/internal-content-api .`

`docker run -ti coco/internal-content-api`

```
docker run -ti  
--env "APP_PORT=8080" \  
--env "ENRICHED_CONTENT_API_URI=http://localhost:8080/__enriched-content-read-api/enrichedcontent/" \  
--env "DOCUMENT_STORE_API_URI=http://localhost:8080/__document-store-api/internalcomponents/" \  
--env "ENRICHED_CONTENT_APP_NAME=Enriched Content Service" \  
--env "DOCUMENT_STORE_APP_NAME=Document Store Service" \  
--env "ENRICHED_CONTENT_APP_HEALTH_URI=http://localhost:8080/__enriched-content-read-api/__health" \  
--env "DOCUMENT_STORE_APP_HEALTH_URI=http://localhost:8080/__enriched-content-read-api/__health" \  
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

The read should return the internal content of an article.

404 if article with given uuid does not exist.

503 when one of the collaborating mandatory services is inaccessible.

### Admin endpoints
Healthchecks: [http://localhost:8084/__health](http://localhost:8084/__health)

Ping: [http://localhost:8084/__ping](http://localhost:8084/__ping)

Build-info: [http://localhost:8084/__build-info](http://localhost:8084/__ping)  -  [Documentation on how to generate build-info] (https://github.com/Financial-Times/service-status-go) 
 
Metrics:  [http://localhost:8084/__metrics](http://localhost:8084/__metrics)
