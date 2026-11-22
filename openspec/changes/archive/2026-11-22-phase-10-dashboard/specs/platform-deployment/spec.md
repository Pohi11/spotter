# Spec Delta

## Purpose

Adds the dashboard to what the chart installs.

## MODIFIED Requirements

### Requirement: One chart declares the platform
The Helm chart SHALL render, with default values, Deployments for the registry, gateway, monitor, inference worker, and dashboard; StatefulSets for Postgres and SeaweedFS; and NATS with JetStream from the official NATS chart. With `postgres.enabled=false`, it SHALL render no Postgres and SHALL take the database URL from an existing Secret. With `dashboard.enabled=false`, it SHALL render no dashboard.

#### Scenario: Default render
- **WHEN** the chart is rendered with default values
- **THEN** it contains the five Deployments, the two StatefulSets, and a NATS StatefulSet with JetStream enabled, and the dashboard's `GATEWAY_URL` points at the gateway Service

#### Scenario: External database
- **WHEN** the chart is rendered with `postgres.enabled=false` and `externalDatabase.existingSecret=db`
- **THEN** it contains no Postgres StatefulSet and the registry reads `DATABASE_URL` from Secret `db`

#### Scenario: Without the dashboard
- **WHEN** the chart is rendered with `dashboard.enabled=false`
- **THEN** it contains no dashboard Deployment or Service
