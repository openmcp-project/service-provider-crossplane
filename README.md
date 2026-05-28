[![REUSE status](https://api.reuse.software/badge/github.com/openmcp-project/service-provider-crossplane)](https://api.reuse.software/info/github.com/openmcp-project/service-provider-crossplane)

# service-provider-crossplane

## About this project

Service provider Crossplane manages the lifecycle of Crossplane instances and Crossplane providers in a `ManagedControlPlane`.

## 🏗️ Installation of the Service Provider Crossplane

### Local Development
To run the service-provider-crossplane locally, you need to first bootstrap an openMCP environment by using [openmcp-operator](https://github.com/openmcp-project/openmcp-operator) and [cluster-provider-kind](https://github.com/openmcp-project/cluster-provider-kind). A comprehensive guide will follow soon.

For current testing reasons, the service-provider-crossplane needs to run in the cluster. To run the latest version of your changes in your local environment, you need to run:

```bash
task build:img:build
```

This will build the image of the service-provider-crossplane locally and puts it into your local Docker registry.

```bash
docker images ghcr.io/openmcp-project/images/service-provider-crossplane
```

You can then apply the `ServiceProvider` resource to your openMCP Platform cluster:

```yaml
apiVersion: openmcp.cloud/v1alpha1
kind: ServiceProvider
metadata:
  name: crossplane
spec:
  image: ghcr.io/openmcp-project/images/service-provider-crossplane:... # latest local docker image build
```

### OpenMCP Landscape
When you already have an openMCP environment set up, you can deploy the service-provider-crossplane by applying the following manifest:

```yaml
apiVersion: openmcp.cloud/v1alpha1
kind: ServiceProvider
metadata:
  name: crossplane
spec:
  image: ghcr.io/openmcp-project/images/service-provider-crossplane:<latest-version> # latest upstream version
```

## 📖 Usage

### Configure a `ProviderConfig`
A `ProviderConfig` is an API where you can configure an allow-list of Crossplane and provider installations in your `ManagedControlPlane`.
The `ProviderConfig` is stored in the Platform cluster and therefore in the responsibility realm of the platform owner.

```yaml
apiVersion: crossplane.services.openmcp.cloud/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  versions:
    - version: v2.0.2
      chart:
        url: "ghcr.io/openmcp-project/openmcp/charts/crossplane:2.0.2" # example OCI regsitry URL for Crosslane helm chart
        secretRef: # optional
          name: ghcr
      image:
        url: "xpkg.crossplane.io/crossplane/crossplane:2.0.2" # currently, upstream location but can be private registry as well
        secretRef:
          name: xyz # optional
    - version: v1.20.0
      chart:
        url: "ghcr.io/openmcp-project/openmcp/charts/crossplane:1.20.0" # example OCI regsitry URL for Crosslane helm chart
        secretRef: # optional
          name: ghcr
      image:
        url: "xpkg.crossplane.io/crossplane/crossplane:1.20.0" # currently, upstream location but can be private registry as well
        secretRef: # optional
          name: xyz

  providers:
    availableProviders:
      - name: provider-kubernetes
        package: xpkg.upbound.io/upbound/provider-kubernetes
        versions:
          - v0.16.0
          - v0.15.0
    imagePullSecretRefs:
      - name: secretforprivateproviders
      - name: xyz
```

The `ProviderConfig` allows you to specify secret references for private helm chart or image locations.
NOTE: `ProvierConfig.spec.versions[].chart.url` needs to be image URL to an OCI registry.

### Install a Crossplane instance

```yaml
apiVersion: crossplane.services.openmcp.cloud/v1alpha1
kind: Crossplane
metadata:
  name: crossplane-sample
  namespace: default
spec:
  version: v1.20.0 # allowed version from ProviderConfig
  providers:
    - name: provider-kubernetes
      version: v0.16.0 # allowed version from ProviderConfig
```

## 📚 Documentation
Extensive documentation for the service-provider-crossplane can be found in the [docs](./docs) folder.

- [Custom CA Bundle Configuration](./docs/configuration/custom-ca.md) - How to configure custom Certificate Authority bundles for Crossplane instances and providers

### Command line options
To get help for available command line options, run:

```bash
go run cmd/service-provider-crossplane/main.go help
```

Service Providers have two main commands: `run` and `init`.
`init` is used to initialize the service provider (e.g. install CRDs at Onboarding API).
`run` is used to start the service provider controller manager.

You can get help for each command individually for what command line arguments you can pass in, e.g.:
```bash
go run cmd/service-provider-crossplane/main.go init --help
```

```bash
go run cmd/service-provider-crossplane/main.go run --help
```

## 🧑‍💻 Development

### Building the binary locally

To build the binary locally, you can use the following command:

```bash
task build
```

### Build the image locally

To build the image locally, you can use the following command:

```bash
task build:img:build
```

### Run unit tests locally

To run the unit tests locally, you can use the following command:

```bash
task test
```

### Generating the CRDs, DeepCopy functions etc.
To generate the CRDs, DeepCopy functions, and other boilerplate code, you can use the following command:

```bash
task generate
```

### Validate the generated code and formatting
To validate the generated code and formatting, you can use the following command:

```bash
task validate
```

## ❤️ Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openmcp-project/service-provider-crossplane/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](https://github.com/openmcp-project/.github/blob/main/CONTRIBUTING.md).

## 🤝 Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/openmcp-project/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## 📋 Licensing

Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmcp-project/service-provider-crossplane).

"Crossplane" is a registered trademark of the Linux Foundation.

<p align="center"><img alt="NeoNephos foundation logo" src="https://raw.githubusercontent.com/neonephos/.github/refs/heads/main/assets/logo.svg" width="400"/></p>

---

<p align="center">
  <a href="https://apeirora.eu/content/projects/">
    <img alt="BMWK-EU funding logo" src="https://apeirora.eu/assets/img/BMWK-EU.png" width="300"/>
  </a>
</p>

<p align="center">
  OpenControlPlane is part of <a href="https://apeirora.eu/content/projects/">ApeiroRA</a>, an EU Important Project of Common European Interest (IPCEI-CIS).
</p>

<p align="center">
  Copyright Linux Foundation Europe. For web site terms of use, trademark policy and other project policies please see <a href="https://linuxfoundation.eu/en/policies">https://linuxfoundation.eu/en/policies</a>.
</p>
