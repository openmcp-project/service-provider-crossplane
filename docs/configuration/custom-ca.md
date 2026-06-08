# Custom CA Bundle Configuration

## Overview

The `CABundleRef` property in the `ProviderConfig` allows you to configure custom Certificate Authority (CA) bundles for Crossplane instances and their providers. This is essential when your infrastructure uses private or self-signed certificates that need to be trusted by Crossplane and its providers.

## Use Cases

- **Private Container Registries**: When pulling Crossplane provider images from registries with self-signed certificates
- **External API Integration**: When Crossplane providers need to communicate with external services using private PKI
- **Air-gapped Environments**: When operating in isolated networks with custom certificate infrastructure

## Configuration

### ProviderConfig Specification

The `caBundleRef` field is an optional reference to a Kubernetes ConfigMap containing your custom CA certificate bundle:

```yaml
apiVersion: crossplane.services.open-control-plane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: default
spec:
  # ... other configuration ...

  caBundleRef:
    name: custom-ca-bundle      # Name of the ConfigMap
    key: ca-bundle.crt          # Key within the ConfigMap containing the certificate bundle
```

### Creating the CA Bundle ConfigMap

First, create a ConfigMap in the Platform cluster containing your CA certificate bundle. The certificate bundle should be in PEM format and can contain multiple certificates concatenated together.

#### Example: Single CA Certificate

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-ca-bundle
  namespace: openmcp-system  # Must be in the same namespace as the service-provider-crossplane
data:
  ca-bundle.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDADCCAeigAwIBAgIUU0jjGMPVbvVbUen942ixQO2k2V4wDQYJKoZIhvcNAQEL
    BQAwGDEWMBQGA1UEAxMNc2VsZnNpZ25lZC1jYTAeFw0yNjAyMDkyMDQ5MTdaFw0y
    ... (certificate content) ...
    -----END CERTIFICATE-----
```

#### Example: Multiple CA Certificates

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-ca-bundle
  namespace: openmcp-system
data:
  ca-bundle.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDADCCAeigAwIBAgIUU0jjGMPVbvVbUen942ixQO2k2V4wDQYJKoZIhvcNAQEL
    ... (first certificate) ...
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    MIIDADCCAeigAwIBAgIUU0jjGMPVbvVbUen942ixQO2k2V4wDQYJKoZIhvcNAQEL
    ... (second certificate) ...
    -----END CERTIFICATE-----
```

#### Creating from a Certificate File

If you have CA certificates in files, you can create the ConfigMap directly:

```bash
kubectl create configmap custom-ca-bundle \
  --from-file=ca-bundle.crt=/path/to/your/ca-bundle.crt \
  --namespace=openmcp-system
```

## How It Works

When you configure a `CABundleRef` in your ProviderConfig:

1. **Installation on ManagedControlPlane**: The CA bundle from the referenced ConfigMap is automatically installed on the `ManagedControlPlane`
2. **Crossplane Runtime Configuration**: The CA bundle is configured for the Crossplane runtime, allowing it to trust the specified certificates
3. **Provider Configuration**: The CA bundle is also configured for all Crossplane providers, ensuring they can communicate with services using your custom certificates
4. **Automatic Propagation**: The service provider automatically handles the propagation and configuration of the CA bundle across all relevant components **except for the container runtime**

**Notice:** You also have to install the CA bundle on your cluster nodes. Otherwise the provider pod will not start because the container runtime cannot pull the provider image.
