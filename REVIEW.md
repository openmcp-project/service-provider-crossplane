# Review: Discoverable Components (`hackathon-discoverable-components`)

## Summary

The change replaces the static, hand-maintained version catalogue in
`ProviderConfigSpec` (`versions[]` + `providers.availableProviders[]`) with a
**runtime discovery mechanism**. A new `DiscoveryReconciler` watches ConfigMaps
on the platform cluster (simulating a future OCM Discovery resource), parses the
published component versions into a shared in-memory `discovery.Store`, and the
`CrossplaneReconciler` now resolves versions against that store instead of the
ProviderConfig.

Overall this is a clean, well-scoped change. Build passes, unit tests pass,
tests are meaningful. The comments are unusually good — they explain intent, not
mechanics. A few correctness gaps around the watch scope and reconcile triggering
are worth addressing before merge.

---

## What's good

- **Store is the right abstraction.** `discovery.Store` is small, `sync.RWMutex`
  guarded, has a concurrency test, and cleanly separates parsing (`types.go` /
  `BuildComponentVersions`) from storage. Ownership tracking (`owned` map) so a
  ConfigMap can *retract* versions it previously published — including empty and
  deleted cases — is the subtle part and it's both implemented and tested
  (`TestDiscoveryReconcile_EmptiedConfigMapClearsStore`, `_CrossplaneDelete`).
- **Resolver simplification.** `GetResolverFunc()` dropped its `*ProviderConfig`
  argument and the branchy Crossplane-vs-provider logic collapsed into a single
  `store.Resolve` + error path. Net negative LOC in the controller.
- **Pull-secret handling reads clearly.** The old
  `extractHelmChartPullSecretForVersion` / `discoverCrossplaneImagePullSecrets`
  pair is replaced by the tiny `crossplanePullSecret` helper. `HasPullSecret`
  keeps secret-change → reconcile mapping working.
- **CRD / deepcopy / samples / e2e all updated consistently.** The dead types
  (`AvailableCrossplaneProvider`, `ChartSpec`, `ImageSpec`, `CrossplaneVersion`)
  are fully removed, deepcopy regenerated.
- **Comments are intent-level and honest** ("simulates the future Discovery
  resource", "parsing errors are not retryable"). Keep this style.

---

## Correctness issues (address before merge)

### 1. `DiscoveryReconciler` never reconciles when the `ProviderConfig` changes
`SetupWithManager` only watches ConfigMaps. If a user edits
`spec.providerDiscoverySelector` or `spec.crossplaneDiscoveryName`, no ConfigMap
event fires, so the store is not re-evaluated until the next unrelated ConfigMap
change. A ConfigMap that *becomes* relevant (or irrelevant) under the new
selector is missed.
→ Add a watch on `ProviderConfig` that enqueues all discovery ConfigMaps (or at
least the named crossplane one + selector matches).

### 2. The ConfigMap watch is cluster/namespace-unscoped
`r.Namespace` is set from `EnvVariablePodNamespace` in `main.go` and documented
as "the namespace on the platform cluster in which the discovery ConfigMaps
live" — **but it is never used**. `WatchesRawSource` watches *every* ConfigMap on
the platform cluster and every one triggers `isRelevant` (which does a
`ProviderConfig` GET each time). Two problems:
- Every ConfigMap change in every namespace wakes this reconciler.
- `isRelevant` matches purely on name/labels, not namespace, so a
  `crossplane-discovery` ConfigMap in *any* namespace would be treated as the
  source.
→ Either use `r.Namespace` to filter the watch (predicate) and the relevance
check, or delete the unused field and document that name/label global matching
is intended. Right now the field is dead code that implies a guarantee the code
doesn't provide.

### 3. Parse-error swallows the reconcile
In `Reconcile`, `applyDiscovery` error path:
```go
if err := r.applyDiscovery(ctx, cm); err != nil {
    return ctrl.Result{}, nil // parsing errors are not retryable
}
```
Returning `nil` (not the error) is correct to avoid a hot requeue loop on bad
YAML — good call. But there is **no surfaced status/event** telling the operator
their discovery ConfigMap is malformed; it's only an `Error` log line. Consider
a Warning event on the ConfigMap or a condition, since a silent parse failure
means "no versions available" with no user-visible cause.

---

## Minor / style

- **`crossplanePullSecret` struct is a one-field wrapper.** `struct{ name string }`
  with `ref()` / `pullSecretRefs()` is slightly more than needed — the same could
  be a plain `string` + two free functions, or just inline. Not worth blocking;
  the methods do read well. (ponytail: borderline over-wrap, leave it.)
- **`AvailableVersions` uses `sort.Strings`** — lexical, not semver. `v1.20.0`
  sorts after `v1.9.0` incorrectly in the error message listing available
  versions. Cosmetic (error text only), but worth a `// ponytail:` note or a
  semver sort if the list is ever shown to users for selection.
- **`Resource.Version` is parsed but unused** in `BuildComponentVersions` (only
  the component-level `Version` is used). Fine if intentional; the field mirrors
  the wire format.
- **`ProviderDiscoverySelector` is `Required`** in the CRD but an empty
  `LabelSelector{}` matches *everything*. A user who supplies an empty selector
  turns every labeled/unlabeled ConfigMap into a provider source. Consider
  validating non-empty or documenting the match-all behavior.
- **`hack/cmdref/main.go`** is a deliberate no-op generator — well commented, fine.
  Unrelated to the feature; flag it in the PR description so reviewers don't
  wonder.
- **`go.mod`**: `sigs.k8s.io/yaml` correctly promoted from indirect to direct
  (now imported in `discovery_controller.go`). Good.

---

## Test coverage

Solid for the happy paths and the tricky retract/delete/empty cases. Gaps:
- No test for a ConfigMap that **changes labels** so it stops matching the
  selector (the `!relevant → releaseOwned` branch).
- No test for `isRelevant` returning an error (invalid selector).
- No end-to-end test in `crossplane_controller_test.go` proving the reconciler
  resolves a version *through the store* (the store is populated directly in
  unit tests; worth one test wiring `VersionStore` + a built component).

---

## Verdict

Direction is right and the implementation is clean and testable.

**Fixed in follow-up commit:**
- #1 — `DiscoveryReconciler` now watches `ProviderConfig` and enqueues all
  discovery-namespace ConfigMaps on change (`enqueueNamespaceConfigMaps`).
- #2 — watch/relevance now scoped to `r.Namespace`; wrong-namespace ConfigMaps
  are ignored in `Reconcile`.
- Empty `providerDiscoverySelector` now matches nothing instead of everything.
- `AvailableVersions` sorts by semver (`Masterminds/semver/v3`), unparseable last.

**Still open (follow-ups):** #3 (silent parse-error has no user-visible
status/event) and an end-to-end resolver-through-store test.
