[![REUSE status](https://api.reuse.software/badge/github.com/openmcp-project/service-provider-crossplane)](https://api.reuse.software/info/github.com/openmcp-project/service-provider-crossplane)

# service-provider-crossplane

## About this project

Service provider Crossplane manages the lifecycle of Crossplane instances.

## Installation of the Service Provider Crossplane

### Local Development

To install the service provider Crossplane locally, you can use the following command:

```bash
TODO
```

### OpenMCP Landscape

```yaml
apiVersion: services.openmcp.cloud/v1alpha1
kind: ServiceProvider
metadata:
  name: crossplane
spec:
  image: ghcr.io/openmcp-project/images/service-provicer-crossplane:<version>
```

## Configure a `ProviderConfig`
A `ProviderConfig` is an API where you can configure the installation of a Crossplane instance in your `ManagedControlPlane`.

```yaml
apiVersion: crossplane.services.openmcp.cloud/v1alpha1
kind: ProviderConfig
metadata:
  name: providerconfig-sample
spec:
  chart:
    repository: "https://charts.crossplane.io/stable"
    name: crossplane
    version: v1.20.0
```

### Install a Crossplane instance

```yaml
apiVersion: crossplane.services.openmcp.cloud/v1alpha1
kind: Crossplane
metadata:
  name: crossplane-sample
  namespace: default
spec:
  version: v1.20.0
  providers:
    - name: provider-kubernetes
      version: v0.18.0
```

## Development

### Building the image locally

To build the image locally, you can use the following command:

```bash
task build
```

### Generating the CRDs, DeepCopy functions etc.
To generate the CRDs, DeepCopy functions, and other boilerplate code, you can use the following command:

```bash
task generate
```

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openmcp-project/service-provider-crossplane/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openmcp-project/service-provider-crossplane/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/openmcp-project/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and service-provider-crossplane contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmcp-project/service-provider-crossplane).

"Crossplane" is a registered trademark of the Linux Foundation.
