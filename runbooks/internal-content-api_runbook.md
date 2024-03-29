# UPP - Internal Content API

This service serves published content that beside the /enrichedcontent fields also includes the internal components of that content.

## Code

up-ica

## Primary URL

<https://upp-prod-delivery-glb.upp.ft.com/__upp-internal-content-api/>

## Service Tier

Platinum

## Lifecycle Stage

Production

## Host Platform

AWS

## Architecture

Internal Content API serves published content that includes the internal components beside the enriched content fields.
The read should return the internal content of an article (i.e. an aggregation of enriched content plus internal components).

## Contains Personal Data

No

## Contains Sensitive Data

No

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

FullyAutomated

## Failover Details

The service is deployed in both Delivery clusters.
The failover guide for the cluster is located here:
<https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster>

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

Manual failover is needed when a new version of
the service is deployed to production.
Otherwise, an automated failover is going to take place when releasing.
For more details about the failover process please see: <https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster>

## Key Management Process Type

Manual

## Key Management Details

To access the service clients need to provide basic auth credentials.
To rotate credentials you need to login to a particular cluster and update varnish-auth secrets.

## Monitoring

Service in UPP K8S delivery clusters:

- Delivery-Prod-EU health: <https://upp-prod-delivery-eu.ft.com/__health/__pods-health?service-name=internal-content-api>
- Delivery-Prod-US health: <https://upp-prod-delivery-us.ft.com/__health/__pods-health?service-name=internal-content-api>

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.
