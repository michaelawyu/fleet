# Hub agent controller Helm Chart

## Install Chart

```console
# Helm install with fleet-system namespace already created
helm install hub-agent ./charts/hub-agent/

# Helm install and create namespace
helm install hub-agent ./charts/hubagent/ --namespace fleet-system --create-namespace
```

## Upgrade Chart

```console
helm upgrade hub-agent ./charts/hubagent/ --namespace fleet-system --create-namespace
```

_See [parameters](#parameters) below._

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._

## Parameters

| Parameter             | Description                                                         | Default                                          |
|:----------------------|:--------------------------------------------------------------------|:-------------------------------------------------|
| replicaCount          | The number of hub-agent replicas to deploy                          | `1`                                              |
| image.repository      | Image repository                                                    | `ghcr.io/azure/azure/fleet/hub-agent`            |
| image.pullPolicy      | Image pullPolicy                                                    | `Always`                                         |
| image.tag             | The image release tag to use                                        | `v0.1.0`                                         |
| namespace             | Namespace that this Helm chart is installed on                      | `fleet-system`                                   |
| serviceAccount.create | Whether to create service account                                   | `true`                                           |
| serviceAccount.name   | Service account name                                                | `hub-agent`                                      |
| resources             | The resource request/limits for the container image                 | limits: 500m CPU, 1Gi, requests: 100m CPU, 128Mi |
| affinity              | The node affinity to use for pod scheduling                         | `{}`                                             |
| tolerations           | The tolerations to use for pod scheduling                           | `[]`                                             |
