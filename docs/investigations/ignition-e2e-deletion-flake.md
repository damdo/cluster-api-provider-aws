# Ignition E2E Flake Investigation: `[unmanaged] [functional] Workload cluster with AWS S3 and Ignition parameter`

## Summary

A recurring e2e flake affects the Ignition/S3 functional test on both `main` and
`release-2.12`. Across 4 sampled `periodic-cluster-api-provider-aws-e2e-release-2-12`
CI runs, the test failed **4/4 times** (100% hit rate in this sample), while all
other 16 tests in each run passed consistently.

Root cause (proven from CloudTrail + controller logs, not assumption): a **race
condition in `AWSMachine` instance creation** causes a duplicate EC2 instance to be
created and orphaned. The orphaned instance's ENI later blocks security group and
subnet deletion, causing the `AWSCluster` deletion to stall until the e2e test's
20-minute timeout expires.

**Update:** the exact proximate trigger for this race has since been identified
with high confidence by reading the code path directly (see
["Proximate trigger" below](#proximate-trigger-of-the-patchobject-failure-confirmed)):
`getIgnitionVersion` mutates `AWSMachine.Spec.Ignition` as a side effect, and the
resulting patch is rejected by the `AWSMachine` validating webhook. A prior fix
for the general shape of this race (PR [#6132](https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/6132),
cherry-picked to `release-2.12` as [#6134](https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/6134))
was already merged **before** all 4 CI runs analyzed here, and is confirmed
**insufficient** — see ["Prior fix attempt" below](#prior-fix-attempt-pr-6132--6134--6138-merged-but-insufficient).

## CI Runs Analyzed

| Run | Date | Failure Phase | Timeout |
|---|---|---|---|
| [`2085558440591101952`](https://prow.k8s.io/view/gs/kubernetes-ci-logs/logs/periodic-cluster-api-provider-aws-e2e-release-2-12/2085558440591101952) | 2026-08-07 | Cluster **creation** — no control plane machine appeared | 2100s (35 min) |
| [`2087008762379374592`](https://prow.k8s.io/view/gs/kubernetes-ci-logs/logs/periodic-cluster-api-provider-aws-e2e-release-2-12/2087008762379374592) | 2026-08-11 | Cluster **deletion** — AWSCluster stuck `Deleting` | 1200s (20 min) |
| [`2087552923138527232`](https://prow.k8s.io/view/gs/kubernetes-ci-logs/logs/periodic-cluster-api-provider-aws-e2e-release-2-12/2087552923138527232) | 2026-08-12 | Cluster **deletion** — AWSCluster stuck `Deleting` | 1200s (20 min) |
| [`2087734239180099584`](https://prow.k8s.io/view/gs/kubernetes-ci-logs/logs/periodic-cluster-api-provider-aws-e2e-release-2-12/2087734239180099584) | 2026-08-13 | Cluster **deletion** — AWSCluster stuck `Deleting` | 1200s (20 min) |

Failing test (all runs):
```
[unmanaged] [functional] Workload cluster with AWS S3 and Ignition parameter [It] It should be creatable and deletable
```
Source: `test/e2e/suites/unmanaged/unmanaged_functional_test.go:658`

In the 3 deletion-timeout runs, the test **passed all functional assertions**
(cluster created, control plane ready, Ignition `MachineDeployment` with
`UnencryptedUserData` validated, S3 endpoint verified) and only failed during
teardown. The `SynchronizedAfterSuite` cleanup then attempted the same deletion
again and also timed out, doubling the wasted CI time (~40-70 min per run).

---

## Root Cause (proven via CloudTrail + controller logs)

### The race: duplicate EC2 instance creation

`controllers/awsmachine_controller.go:584-612` (`reconcileNormal`) has no
idempotency guard around instance creation:

```go
if instance == nil {
    ...
    instance, err = r.createInstance(ctx, ec2svc, machineScope, clusterScope, objectStoreSvc)
    if err != nil { ... }

    if patchErr := machineScope.PatchObject(); patchErr != nil {
        machineScope.Error(patchErr, "failed to patch providerID")
        return ctrl.Result{}, patchErr   // <-- providerID never persisted
    }
}
```

`GetRunningInstanceByTags` (`pkg/cloud/services/ec2/instances.go:52-83`) is used
to look up an existing instance when the providerID is empty. It has a standing
`TODO` acknowledging the underlying design gap:

```go
// TODO: currently just returns the first matched instance, need to
// better rationalize how to find the right instance to return if multiple
// match
```

**Observed sequence (from run 4's controller logs, `2087734239180099584`):**

| Time | Event |
|---|---|
| `03:09:34.048` | Reconcile #1: `GetRunningInstanceByTags` → nil (no instance yet) |
| `03:09:36.736` | `RunInstances` → `i-04c32498cb29a838d` created |
| `03:09:37.164` | `PatchObject()` **fails**: admission webhook rejects the AWSMachine spec patch (`"cannot be modified"`) → reconciler returns error → **immediate requeue** |
| `03:09:37.175` | Reconcile #2 starts 11ms later. `GetRunningInstanceByTags` → **nil** (EC2 API eventual consistency; the instance created 0.4s earlier isn't visible yet by tag query) |
| `03:09:39.767` | `RunInstances` → **`i-03c1eaa63467b74f6` created (duplicate)** |
| `03:09:40.171` | `PatchObject()` fails again → requeue |
| `03:09:40.177` | Reconcile #3: `GetRunningInstanceByTags` → now finds `i-04c32498cb29a838d` (consistency caught up) → controller adopts this instance |
| — | **`i-03c1eaa63467b74f6` is never referenced again by the controller** |

Confirmed independently via CloudTrail (runs 3 and 4, same underlying job data):

```
RunInstances  03:09:36Z  i-04c32498cb29a838d   Name=functional-test-ignition-h2572l-control-plane-gp6jh
RunInstances  03:09:39Z  i-03c1eaa63467b74f6   Name=functional-test-ignition-h2572l-control-plane-gp6jh  (same clientToken)
TerminateInstances 03:14:49Z  i-04c32498cb29a838d   (only this one)
i-03c1eaa63467b74f6: NEVER terminated — still running at 03:48Z (active SSM heartbeats)
```

Both instances shared the same `clientToken`, the same `Name` tag, and the same
security groups (`lb`, `controlplane`, `node`) in `subnet-05740af0c6ccfc38d`.

> Note: the "same `clientToken`" observation above came from a sub-agent's
> CloudTrail summary and was not independently re-verified byte-for-byte
> against the raw event data. `RunInstancesInput` is not currently constructed
> with an explicit `ClientToken` anywhere in this codebase
> (`pkg/cloud/services/ec2/instances.go:586-595`), so if the AWS SDK generates
> one implicitly per call, two separate `RunInstances` calls would ordinarily
> get *different* auto-generated tokens. This detail doesn't affect the root
> cause analysis below (which was independently confirmed via code reading,
> not CloudTrail), but flagging it so it isn't taken as fact elsewhere.

### Proximate trigger of the `PatchObject` failure (confirmed)

All 3 deletion-stall runs show the immediate `PatchObject()` call after
`createInstance` (`awsmachine_controller.go:608-611`) failing with:

```
admission webhook "validation.awsmachine.infrastructure.cluster.x-k8s.io" denied the request:
AWSMachine.infrastructure.cluster.x-k8s.io "..." is invalid: spec: Forbidden: cannot be modified
```

This was initially an open question (see the original "Follow-up Items" list),
but has since been confirmed by reading the code directly — no further log or
CloudTrail digging was needed.

`getIgnitionVersion` (`awsmachine_controller.go:960-968`) is a getter-shaped
function that mutates spec as a side effect:

```go
func getIgnitionVersion(scope *scope.MachineScope) string {
	if scope.AWSMachine.Spec.Ignition == nil {
		scope.AWSMachine.Spec.Ignition = &infrav1.Ignition{}   // mutates spec
	}
	if scope.AWSMachine.Spec.Ignition.Version == "" {
		scope.AWSMachine.Spec.Ignition.Version = infrav1.DefaultIgnitionVersion  // mutates spec
	}
	return scope.AWSMachine.Spec.Ignition.Version
}
```

It's called from `generateIgnitionWithRemoteStorage` (line 899), which only
runs when Ignition uses the **default `ClusterObjectStore` storage type** —
exactly what the ignition e2e template's control-plane and `md-0` worker
machines use (their `AWSMachineTemplate`s don't set `spec.ignition` at all).

Sequence within a single `reconcileNormal` call:

1. `createInstance` → `resolveUserData` → `generateIgnitionWithRemoteStorage` →
   `getIgnitionVersion` mutates `scope.AWSMachine.Spec.Ignition` from `nil` to
   `&Ignition{Version: "3.4.0"}` **in memory**.
2. `RunInstances` succeeds.
3. `PatchObject()` (line 608, added by PR #6132 — see below) persists the
   whole object, including the now-non-nil `spec.ignition`.
4. The webhook's `ValidateUpdate` (`webhooks/awsmachine_webhook.go:93-172`)
   diffs old vs. new spec after explicitly exempting `providerID`,
   `instanceID`, `additionalTags`, `additionalSecurityGroups`, and a couple of
   `cloudInit`/`privateDnsName` sub-fields (lines 127-165) — but **not**
   `ignition`. The diff is non-empty, so the patch is rejected
   (`awsmachine_webhook.go:167-169`).
5. The rejection error propagates out of `reconcileNormal` and triggers an
   immediate requeue — the start of the race documented above.

This is precisely and only triggered by Ignition machines using the default
`ClusterObjectStore` storage type, which matches every observed data point:
in every run, the orphaned instance was the **control-plane** machine
(default storage type); the `md-unencrypted-userdata` worker — which
explicitly sets `UnencryptedUserData` and never calls
`generateIgnitionWithRemoteStorage` — was never the orphan.

### Prior fix attempt: PR #6132 / #6134 / #6138 (merged, but insufficient)

[PR #6132](https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/6132)
("Persist ProviderID immediately after instance creation", fixing issue #6131)
diagnosed and fixed the *general* shape of this race: the deferred `Close()`
patch at the end of `reconcileNormal` can fail (e.g. on an optimistic-locking
conflict from the finalizer/condition patches earlier in the same reconcile),
leaving `providerID` unpersisted; the next reconcile then misses the instance
via `GetRunningInstanceByTags` due to EC2 `DescribeInstances` eventual
consistency, and creates a duplicate. The fix adds exactly the immediate
`PatchObject()` call at `awsmachine_controller.go:608-611` referenced above.

This PR was merged to `main` on 2026-07-20 and cherry-picked to `release-2.12`
as [#6134](https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/6134)
(merged 2026-07-22) and to `release-2.11` as
[#6138](https://github.com/kubernetes-sigs/cluster-api-provider-aws/pull/6138).

**All 4 CI runs analyzed in this document (2026-08-07, 08-11, 08-12, 08-13)
postdate the `release-2.12` merge by 2+ weeks.** The fix was live on the
branch for every failure documented here, and the flake still occurred —
because the *immediate* `PatchObject()` call #6132 added is itself the call
that fails, for a different reason (the `ignition` webhook rejection above)
than the one #6132 targeted (optimistic-locking conflicts). #6132 correctly
hardens the general case but does not close this specific trigger.

### The cascade: orphaned ENI blocks cluster deletion

During cluster teardown, `AWSMachine.reconcileDelete` (`awsmachine_controller.go:340`,
`findInstance`) looks up the instance by **providerID** (once persisted) or by
tags. Either way it only ever finds `i-04c32498cb29a838d` — the orphan is
invisible to deletion logic just as it was invisible to creation logic. Only the
known instance gets terminated; the orphan keeps running with its ENI
(`eni-06e5bb4c860a659db`) attached to 3 cluster security groups.

The `AWSCluster` controller (`controllers/awscluster_controller.go:218-303`) then
tries to tear down the VPC. Deletion order:

```
1. S3 Bucket        — OK
2. Load Balancers   — OK (NLB + its 3 ENIs deleted within 25s)
3. Bastion          — OK
4. Security Groups  — FAILS: DependencyViolation (orphan ENI still attached)
5. GC (tag-based)   — discovers the orphan instance by tag, but has no handler
                       for EC2 instance ARNs — can only delete LBs/target
                       groups/SGs (pkg/cloud/services/gc/cleanup.go)
6. Network          — 4-5 of 6 subnets delete fine; the orphan's subnet FAILS:
                       DependencyViolation
```

`pkg/cloud/services/network/enis.go:33-89` (`deleteOrphanedENIs`) exists
specifically to clean up leftover ENIs before subnet deletion, but its
`DescribeNetworkInterfaces` filter only looks for `status=available`:

```go
{
    Name:   aws.String("status"),
    Values: []string{"available"},
},
```

The orphan instance is still running, so its ENI is `status=in-use` — invisible
to this cleanup path. Confirmed via CloudTrail: **409 `DescribeNetworkInterfaces`
calls, 100% filtered on `status=available`, zero ever matched the orphan ENI.**

### The stall

With the SG/subnet deletion permanently failing, `reconcileDelete` returns an
aggregate error every cycle and requeues almost immediately (no meaningful
backoff observed — cycles ran every ~5-7 seconds). This repeated for the
duration of each run:

| Run | Retry cycles | Duration | DependencyViolation errors |
|---|---|---|---|
| `…4592` | ~240 @ 6s | 16 min | recorded on 2 of 4 SGs |
| `…7232` | ~454 @ 5s | 36 min | 1,641 across 3 SGs + 411 on subnet |
| `…9584` | ~134-454 @ 5-7s | 18-40 min (2 controller pods) | 1,641 across 3 SGs + 411 on subnet |

The e2e test's `WaitForClusterDeleted` gives up after 1200s (20 min), well before
the underlying condition would ever resolve on its own (the orphan instance is
never going away without manual/automatic termination).

---

## Why This Manifests on the Ignition Test Specifically

**Confirmed** (see "Proximate trigger" above): the race is triggered
specifically by Ignition machines using the default `ClusterObjectStore`
storage type. `getIgnitionVersion`'s side-effect mutation of
`spec.ignition` only happens on that code path
(`generateIgnitionWithRemoteStorage`), and the `AWSMachine` webhook doesn't
exempt `spec.ignition` from its immutability check the way it exempts
`providerID`/`instanceID`/etc. Cloud-init-based tests never touch this code
path at all, so they're structurally immune to this specific trigger — which
is why the flake concentrates almost entirely on this one test.

It remains possible that the *general* race (patch failing for some other
reason, e.g. a genuine optimistic-locking conflict) affects other tests at a
much lower frequency, since that shape of race isn't Ignition-specific — see
Follow-up item on broader log sweep.

## Run 1 (creation-phase failure) — separate, unconfirmed

Run `2085558440591101952` failed differently: no control plane machine ever
appeared within 35 minutes. We did not pull controller logs for this run. It's
plausible this is the *same* underlying race manifesting differently (e.g. both
duplicate instances failing to become visible/healthy, or a stuck webhook loop
preventing any instance from ever being adopted), but this is **speculation,
not confirmed** — flagged as a follow-up.

---

## Ruled Out

Investigated and **not** the cause, despite initial suspicion:

- **S3 bootstrap data cleanup** — S3 objects and buckets were deleted
  successfully (or already gone) before the stall began in every run examined.
  `s3.go`'s lack of retries and the `BestEffortDeleteObjects` default are real
  gaps but were not implicated in these 3 failures.
- **NLB/ELB ENI lingering** — the NLB and its ENIs were fully cleaned up by AWS
  within ~25 seconds of `DeleteLoadBalancer`, well within the retry window. The
  `apiserver-lb` security group (only referenced by the NLB) deleted
  successfully every time.
- **Security group cross-referencing / ingress rule ordering** — ingress rules
  were successfully revoked from all SGs before deletion attempts in every run.
- **Bootstrap secret race deleting S3 objects before machine cleanup** — a
  real code path (`awsmachine_controller.go:970-978`, `UseIgnition("")`
  returning false when the bootstrap secret is already gone) but not what
  caused these specific stalls; the S3 bucket was gone before the stalls began.
- **Security-group-deletion-before-network-deletion ordering** — initially
  suspected that `AWSCluster.reconcileDelete` runs `sgService.DeleteSecurityGroups()`
  (step 4) before `networkSvc.DeleteNetwork()` (step 6, which contains the
  orphaned-ENI cleanup), and that this ordering alone could cause a stall. In
  practice this doesn't matter: errors are aggregated
  (`awscluster_controller.go:268-299`), so *all* steps run every reconcile
  cycle regardless of earlier failures, and the loop retries every ~5-7s. The
  real defect isn't step ordering — it's that `deleteOrphanedENIs` never finds
  the orphan ENI at all (see Suggested Fix #2), so re-ordering the steps
  would not have helped.

---

## Suggested Fixes

### 1. (Primary, confirmed root cause) Stop `getIgnitionVersion` from mutating spec

This directly removes the trigger identified above, and is what actually
needs to happen for #6132's fix to work as intended on Ignition/`ClusterObjectStore`
machines.

**Files:** `controllers/awsmachine_controller.go` (+ a unit test, e.g. in
`controllers/awsmachine_controller_test.go`)

Make `getIgnitionVersion` a pure getter — no spec mutation:

```go
func getIgnitionVersion(scope *scope.MachineScope) string {
	if scope.AWSMachine.Spec.Ignition != nil && scope.AWSMachine.Spec.Ignition.Version != "" {
		return scope.AWSMachine.Spec.Ignition.Version
	}
	return infrav1.DefaultIgnitionVersion
}
```

`generateIgnitionWithRemoteStorage` (lines 935, 945) currently reads
`scope.AWSMachine.Spec.Ignition.Proxy` / `.TLS` directly, relying on the
getter's side effect to guarantee `Spec.Ignition` is non-nil by that point.
Since it can now legitimately still be `nil`, those two reads need a nil
guard on `scope.AWSMachine.Spec.Ignition` itself before dereferencing
`.Proxy` / `.TLS`.

This is narrowly scoped, has no interaction with AWS-side idempotency
semantics, and is a pure bugfix (a getter should not have side effects) — it
doesn't need a design discussion the way the options below do.

### 1b. (Already attempted, insufficient) Make `AWSMachine` instance creation idempotent

The core issue this addresses is that `createInstance` can run twice for the
same `AWSMachine` if the post-creation `PatchObject()` fails. Options, roughly
in order of robustness vs. invasiveness:

- **Don't discard a successful instance creation on patch failure.** ✅
  **Already implemented** by PR #6132/#6134/#6138 (see above): if
  `RunInstances` succeeds but the deferred end-of-reconcile `PatchObject`
  might fail, persist the providerID via an immediate `PatchObject()` call
  right after `createInstance` returns. **Confirmed insufficient on its own**
  — this immediate patch can itself fail (as it does for the Ignition
  trigger above), and when it does, the same race recurs. Fix #1 (above)
  closes the specific gap in `release-2.12`/`main` today; this bullet is kept
  for context on what's already been tried.
- **Tag-and-check before create, with a stronger lookup.** Not yet
  implemented. Immediately after a `PatchObject` failure, re-query more
  aggressively (e.g. retry `GetRunningInstanceByTags` with a short backoff),
  or fall back to `ClientToken`-based idempotency: EC2's `RunInstances`
  supports `ClientToken` for exactly this kind of duplicate-prevention — pass
  a deterministic token derived from the `AWSMachine` UID so a retried
  `RunInstances` call is a no-op against AWS itself rather than relying on
  tag-query visibility. Worth keeping as defense-in-depth for *other*,
  not-yet-identified triggers of `PatchObject` failure after instance
  creation, but has real edge cases (see below) and shouldn't be the primary
  fix given #1 above closes the actually-observed trigger.
  - Caveat identified during design discussion: EC2's `RunInstances`
    idempotency token has a bounded window (documented as up to ~1 hour); if
    an `AWSMachine`'s instance is legitimately terminated and needs
    replacing again within that window, reusing the same static per-machine
    token could return the terminated instance's stale details instead of
    launching a genuinely new one. This isn't a blocker, but means the token
    should probably not be a purely static UID-derived value if this
    approach is pursued — flagged here so it isn't a surprise later.
- **Fix `GetRunningInstanceByTags`'s multi-match TODO** (`instances.go:73-75`):
  if a tag query ever returns more than one instance for the same
  `AWSMachine`, that's itself a signal of exactly this bug. At minimum, log a
  warning/emit an event when multiple matches are found so this doesn't
  silently orphan instances in the future; ideally, pick the oldest/first-
  launched deterministically and flag the rest for cleanup rather than
  ignoring them.

### 2. (Defense in depth) Fix orphaned ENI cleanup to catch `in-use` ENIs

`pkg/cloud/services/network/enis.go`'s `deleteOrphanedENIs` only queries
`status=available`. Extend it to also detect ENIs that are `in-use` but attached
to instances that no longer correspond to any known `AWSMachine`/`Machine` in
the cluster (i.e., cross-reference `DescribeNetworkInterfaces` results'
`Attachment.InstanceId` against currently-tracked machines, or simply against
all EC2 instances tagged with the cluster's `sigs.k8s.io/cluster-api-provider-aws/cluster/<name>`
tag that aren't in an expected state). This would have caught and self-healed
the exact orphan instance seen in all 3 deletion stalls, independent of whether
fix #1 lands.

### 3. (Defense in depth) Teach the GC service to delete orphaned EC2 instances

`pkg/cloud/services/gc/cleanup.go` already discovers tagged-but-unmanaged
resources via the Resource Groups Tagging API during cluster deletion (this is
how it found the orphan instance's ARN in the logs) but has no handler for
`ec2:instance` ARNs — it silently skips them ("Resource type does not match").
Adding an EC2 instance handler that terminates instances tagged with the
cluster-owned tag but not tracked by any `AWSMachine` would provide a robust
last-resort cleanup path, independent of fixes #1/#2. Note: `ExternalResourceGC`
is not enabled by default — this fix would only help when that feature gate is
on, so it should be paired with #1 and/or #2 rather than relied on alone.

### 4. (Minor / lower priority) S3 delete hardening

Not implicated in these specific failures, but noticed along the way and worth
tracking separately:

- `pkg/cloud/services/s3/s3.go`'s `Delete`/`deleteObject` have no retry/backoff
  for transient errors, and a hard S3 error blocks `AWSMachine` deletion
  entirely (`awsmachine_controller.go:333-337` returns immediately on any
  `deleteBootstrapData` error, before even attempting EC2 instance
  termination). Consider decoupling S3 cleanup failures from instance
  termination (attempt both, aggregate errors, the way `AWSCluster.reconcileDelete`
  already does) so a flaky S3 call doesn't block instance teardown.
- `DeleteBucket` only cleans the `machine-pool/` prefix; per-machine objects
  under `control-plane/`/`node/` are expected to be cleaned by the AWSMachine
  controller. If that cleanup is skipped (e.g., bootstrap secret already
  deleted, see `awsmachine_controller.go:970-978`), the bucket silently leaks
  (`BucketNotEmpty` is swallowed as non-error in `s3.go:181-182`). Low priority
  since it doesn't block deletion, just leaks a bucket.
- The ignition e2e test itself
  (`test/e2e/suites/unmanaged/unmanaged_functional_test.go:657-753`) has no
  `defer shared.DumpSpecResourcesAndCleanup(...)` safety net, unlike most other
  tests in the same file. Adding one would prevent additional resource leakage
  if this test fails before reaching its explicit `deleteCluster` call.

---

## Follow-up Items

1. ~~Identify the exact webhook rejection cause.~~ **Resolved** — see
   "Proximate trigger of the `PatchObject` failure" above:
   `getIgnitionVersion` mutates `spec.ignition`, which the webhook doesn't
   exempt from its immutability check.
2. **Confirm whether run 1's creation-phase failure shares this root cause.**
   Was not investigated at the controller-log level.
3. **Check whether this affects other unmanaged tests at lower frequency.**
   The *general* race (patch failing for reasons other than the `ignition`
   trigger, e.g. a genuine optimistic-locking conflict) isn't Ignition-specific
   and isn't fully closed by fix #1; worth a broader log sweep across
   non-Ignition test failures to see if the same duplicate-instance signature
   appears intermittently elsewhere.
4. **Run 2's CloudTrail artifact could not be independently retrieved** — the
   fetched file appears to have been the same underlying data as runs 3/4
   (same cluster name `functional-test-ignition-h2572l` came back). Run 2's
   controller logs independently confirm the identical SG/subnet
   `DependencyViolation` stall pattern, but the duplicate-`RunInstances` trigger
   was not independently re-verified via that run's own CloudTrail data.
5. **Audit other getter-shaped functions for similar accidental spec-mutation
   side effects.** `getIgnitionVersion` mutating `AWSMachine.Spec.Ignition` as
   a side effect is the kind of bug that's easy to reintroduce elsewhere
   (any `get*`/`resolve*`-named helper that lazily defaults a spec field
   in-place instead of returning a value). Worth a quick sweep of similar
   patterns in `controllers/awsmachine_controller.go` and
   `pkg/cloud/scope/machine.go` once fix #1 lands.
6. **Verify the fix in CI.** Once fix #1 is implemented, confirm via a soak
   run or repeated CI triggers of the Ignition functional test (and ideally a
   targeted unit test asserting `getIgnitionVersion` does not mutate
   `AWSMachine.Spec.Ignition`) that the flake no longer reproduces.

---

## Artifact Locations (for reproducing this investigation)

All artifacts live under
`https://storage.googleapis.com/kubernetes-ci-logs/logs/periodic-cluster-api-provider-aws-e2e-release-2-12/<build-id>/`.
Useful paths within that prefix:

- `build-log.txt` — Ginkgo test output, failure messages, stack traces.
- `finished.json` — pass/fail, timestamps.
- `artifacts/clusters/bootstrap/controllers/capa-controller-manager/<pod>/manager.log`
  — CAPA controller manager logs. **Note:** the controller pod is often
  redeployed mid-run (two different pod names/ReplicaSet hashes appear), so
  check all pods under this directory, not just one.
- `artifacts/cloudtrail-events.yaml` — raw AWS CloudTrail events for the whole
  test account/run. Large (tens of MB); best searched with targeted
  grep/filter rather than loaded whole. This is what let us confirm the
  duplicate `RunInstances` calls and the `status=available`-only
  `DescribeNetworkInterfaces` filter pattern.
- `artifacts/clusters-afterDeletionTimedOut/<cluster-name>/resources/<namespace>/AWSCluster/<cluster-name>.yaml`
  — dump of the AWSCluster object (with conditions) at the moment the test
  gave up. Useful for confirming which conditions were stuck (`ClusterSecurityGroupsReady`,
  `SubnetsReady`, etc.) without re-parsing logs.
- `artifacts/clusters-afterDeletionTimedOut/<cluster-name>/resources/<namespace>/Cluster/clusterctl-describe-cluster-<cluster-name>.txt`
  — `clusterctl describe` snapshot at timeout.

## Reproduction Signature (grep patterns)

To quickly confirm a new CI failure is *this* bug rather than a new flake:

- In `build-log.txt`: `waiting for cluster deletion timed out` +
  `condition: Deleting` + `message: Waiting for AWSCluster to be deleted`,
  specifically on the Ignition functional test.
- In `manager.log`: repeated `"Reconciler error"` lines containing
  `DependencyViolation: resource sg-... has a dependent object` for 3 of the 4
  cluster security groups (`lb`, `node`, `controlplane` — `apiserver-lb`
  usually clears fine), plus `DependencyViolation: The subnet '...' has
  dependencies and cannot be deleted` for exactly one subnet.
- In `cloudtrail-events.yaml`: two `RunInstances` events with the same `Name`
  tag and same `clientToken` within a few seconds of each other, where only
  one of the resulting instance IDs ever gets a matching `TerminateInstances`.
