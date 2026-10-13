# Poll Interval Configuration

## Overview

The poll interval controls how often a Crossplane provider checks for drift between the desired and actual state of managed resources. By default Crossplane uses its own built-in interval, but a platform owner can override this via the `ProviderConfig`.

## Configuring the Poll Interval

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

When a `pollInterval` is specified and the provider's `DeploymentRuntimeConfig` does not yet exist (for example because the provider was not deployed before), the service provider writes a `--poll=<duration>` argument to the `package-runtime` container:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  name: provider-kubernetes
spec:
  deploymentTemplate:
    spec:
      template:
        spec:
          containers:
          - name: package-runtime
            args:
            - --poll=10m0s
```
> [!NOTE]
> Once the `DeploymentRuntimeConfig` exists, the service provider preserves whatever `args` are present on the `package-runtime` container on every reconcile. This means the `--poll` value can only be adjusted manually after the initial creation of the `DeploymentRuntimeConfig`. Updating `pollInterval` in the `ProviderConfig` after the `DeploymentRuntimeConfig` has already been created has no effect.

## Duration Format

`pollInterval` accepts standard Go duration strings, e.g. `30s`, `5m`, `1h`.
