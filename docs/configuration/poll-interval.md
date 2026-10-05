# Poll Interval Configuration

## Overview

The poll interval controls how often a Crossplane provider checks for drift between the desired and actual state of managed resources. By default Crossplane uses its own built-in interval but platform owners as well as end users can override this.

## Ownership Tracking

The `DeploymentRuntimeConfig` has the annotation `open-control-plane.io/managed-poll-interval` to track whether the poll interval should be managed by the service provider, if set to `true`, or an end user, if set to `false`.

## Configuring the Poll Interval - Platform Owner

A platform owner can set `pollInterval` on an entry in `spec.providers.availableProviders` inside the `ProviderConfig`:

```yaml
apiVersion: services.open-control-plane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: my-provider-config
spec:
  providers:
    availableProviders:
      - name: provider-kubernetes
        package: xpkg.upbound.io/upbound/provider-kubernetes
        versions:
          - v0.16.0
          - v0.15.0
        pollInterval: 10m
```

When a `pollInterval` is specified, the service provider writes a `--poll=<duration>` argument to the `package-runtime` container of the provider's `DeploymentRuntimeConfig`.

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  annotations:
    open-control-plane.io/managed-poll-interval: "true" # set to true for automated management
  name: provider-kubernetes
spec:
  deploymentTemplate:
    spec:
      selector: {}
      strategy: {}
      template:
        metadata: {}
        spec:
          containers:
          - name: package-runtime
            args:
            - --poll=5m0s
```

## Configuring the Poll Interval - End User

An end user can take manual control of the poll interval by setting the annotation `open-control-plane.io/managed-poll-interval` to `false` and providing a `--poll` argument directly on the provider's `DeploymentRuntimeConfig`. The service provider treats the value as manually set and preserves it on every reconcile, ignoring the value from the `ProviderConfig`.

```yaml
```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  annotations:
    open-control-plane.io/managed-poll-interval: "false" # set to false for manual override
  name: provider-kubernetes
spec:
  deploymentTemplate:
    spec:
      selector: {}
      strategy: {}
      template:
        metadata: {}
        spec:
          containers:
          - name: package-runtime
            args:
            - --poll=1m0s
```
```

## Duration Format

`pollInterval` accepts standard Go duration strings, e.g. `30s`, `5m`, `1h`.
