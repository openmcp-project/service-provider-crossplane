# How to re-create the environment?

1. Kind create cluster (Platform) -> export Kubeconfig
2. Kind create cluster (Onboarding) -> export Kubeconfig
3. Kind create cluster (MCP) -> export Kubeconfig and store as Secret in cluster (-> like the `AccessRequest` resource would create it)
4. Install Flux on Platform cluster -> Platform Service -> Assume to be there
5. Start sp-crossplane **init** --kubeconfig onboarding --kubeconfig platform --environment development
   1. Installs `ProviderConfig` on platform cluster
   2. Installs `Crossplane` on Onboarding Cluster
6. Start sp-crossplane **run** --kubeconfig onboarding --kubeconfig platform --environment development


## How does the flow work?
1. User creates `ProviderConfig` with available crossplane versions and providers and their versions (explicit)
2.