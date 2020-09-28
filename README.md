[![Circle CI](https://circleci.com/gh/Financial-Times/internal-content-api.svg?style=shield)](https://circleci.com/gh/Financial-Times/internal-content-api)[![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/internal-content-api)](https://goreportcard.com/report/github.com/Financial-Times/internal-content-api) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/internal-content-api/badge.svg?branch=master)](https://coveralls.io/github/Financial-Times/internal-content-api?branch=master)

# Internal Content API (internal-content-api)

__Internal Content API serves published content that includes the internal components beside the enriched content fields__

## Installation

For the first time:

```bash
go get -u github.com/Financial-Times/internal-content-api
cd $GOPATH/src/github.com/Financial-Times/internal-content-api
dep ensure
go build .
```

## Running

## Running locally

### How can I run it

1. Run the tests and install the binary:

        dep ensure
        go test -race ./...
        go install
2. Run the binary locally with properties set:

```bash
go install
$GOPATH/bin/internal-content-api \
--app-port  "8084" \
--handler-path "internalcontent" \
--content-source-uri "http://localhost:8080/__enriched-content-read-api/enrichedcontent" \
--internal-components-source-uri "http://localhost:8080/__content-public-read/internalcontent" \
--content-source-app-name  "Content Source Service" \
--internal-components-source-app-name  "Internal Components Source Service" \
--content-source-app-health-uri  "http://localhost:8080/__enriched-content-read-api/__health" \
--internal-components-source-app-health-uri  "http://localhost:8080/__content-public-read/__health" \
--content-unroller-app-name "Content Unroller" \
--content-unroller-uri "http://localhost:8080/__content-unroller-api/internalcontent" \
--content-unroller-app-health-uri "http://localhost:8080/__content-unroller-api/__health" \
```

* CI provided by CircleCI: [internal-content-api](https://circleci.com/gh/Financial-Times/internal-content-api)

When deployed locally arguments are optional.

## Endpoints

### GET

/internalcontent/{uuid}
Example
`curl -v http://localhost:8084/internalcontent/9358ba1e-c07f-11e5-846f-79b0e3d20eaf`

The read should return the internal content of an article (i.e. an aggregation of enriched content plus internal components).

#### Optional Parameters

`unrollContent={boolean}`, default *false*

When `true` dynamic content, main image, body embedded images, lead images and alternative images get expanded with the content as content-public-read service was called for that dynamic component. This service uses content-unroller which is responsible to get the requested dynamic components.
When `false` the response contains only the IDs of the dynamic content and images (main image, body embedded images, lead images and alternative images).

`404` if article with given uuid does not exist.

`503` when one of the collaborating mandatory services is inaccessible.

### Admin endpoints

Healthchecks: [http://localhost:8084/__health](http://localhost:8084/__health)

good-to-go: [http://localhost:8084/__gtg](http://localhost:8084/__gtg)

Health and gtg are based on enriched-content-read-api and content-public-read's health endpoints availability.

Ping: [http://localhost:8084/__ping](http://localhost:8084/__ping)

Build-info: [http://localhost:8084/__build-info](http://localhost:8084/__build-info)  -  [Documentation on how to generate build-info] (https://github.com/Financial-Times/service-status-go) 
Metrics:  [http://localhost:8084/__metrics](http://localhost:8084/__metrics)

## Model

For the model spec please refer to:

* [enriched-content-read-api](https://github.com/Financial-Times/enriched-content-read-api) - returns enriched content (content + complementary content + annotations + relations)

* [content-public-read](https://github.com/Financial-Times/content-public-read) - it allows public access to content data provided by the platform

* [content-unroller](https://github.com/Financial-Times/content-unroller) - expands images and dynamic content of an article
