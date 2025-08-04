# How to re-create the environment?

1. Kind create cluster (Platform) -> export Kubeconfig
2. Kind create cluster (Onboarding) -> export Kubeconfig
3. Kind create cluster (MCP) -> export Kubeconfig and store as Secret in cluster (-> like the `AccessRequest` resource would create it)
4. Install Flux on Platform cluster -> Platform Service -> Assume to be there
5. Start sp-crossplane **init** --kubeconfig onboarding --kubeconfig platform --environment development
   1. Installs `ProviderConfig` CRD on platform cluster
   2. Installs `Crossplane` CRD on Onboarding Cluster
6. Start sp-crossplane **run** --kubeconfig onboarding --kubeconfig platform --environment development


## How does the flow work?
1. Platform owner creates `ProviderConfig` with available Crossplane versions and providers and their versions (explicit)
2. User creates a `Crossplane` resource in the Onboarding cluster
3. The `Crossplane` resource is reconciled by the `sp-crossplane` controller
    1. It fetches the `ProviderConfig` from the Platform cluster
    2. It checks if the requested Crossplane version is available
    3. It checks if the requested providers and their versions are available
    4. It creates a Crossplane instance in the MCP cluster