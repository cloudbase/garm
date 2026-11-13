# Experimental VM resource admission

This experimental mode accounts for VM creation on one enrolled provider. It
does **not** gate GitHub job assignment or `AcquireJobs`, establish failover,
or guarantee that a job can start. The configured `jobs` units are charged per
VM allocation; they do not track actual assigned or running jobs. GitHub's
automatic-assignment and legacy acquisition semantics require separate
verification.

The initial shape is one controller and one provider per exclusive domain;
all enrolled scale sets must use the same project, exact local image
fingerprint, profile, architecture and resource vector. Min-idle must be zero.
Pools using the enrolled provider are rejected. Outside workloads and other
controllers must be covered by a separately reserved envelope. The operator
must explicitly approve host capability; health probes validate eligibility,
not future create success. Only a provider implementing the experimental
`readiness_v1` command and enforcing `admission_resources` can participate.

A new domain starts drained. After inventory and two fresh readiness probes,
an explicit `activate_generation` equal to `generation` permits activation.
A fault or administrative drain persists across restart; resuming it requires a
higher configuration and activation generation. A budget decrease blocks new
reservations and does not evict existing VMs. Ambiguous create or delete
outcomes retain their charges and require reconciliation; elapsed time,
health expiry and database row removal do not establish absence. Automatic
release requires successful identity-bound deletion and the provider's
versioned absence confirmation. Other providers and uncertain operations
remain charged.

The enrolled external provider must set `exec_timeout_seconds` between 1 and
1200. This bounds provider calls; admitted delete has its own two-minute limit.
For example, add `exec_timeout_seconds = 1200` under the enrolled provider's
external configuration.

Example, with synthetic values (replace every value for the deployment):

```toml
[admission]
enabled = true
protocol = "vm-only-experimental"
domain = "ci-host"
provider = "incus"
generation = 1
activate_generation = 1
project = "ci"
image = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
flavor = "ci-profile"
os_arch = "amd64"
instance_type = "virtual-machine"
exclusive_controller = true
host_capability_approved = true
external_envelope_reserved = true

[admission.capacity]
cpu = 16
memory_bytes = 34359738368
disk_bytes = 214748364800
vms = 4
jobs = 4

[admission.runner]
cpu = 4
memory_bytes = 8589934592
disk_bytes = 53687091200
vms = 1
jobs = 1
```

Do not use this experimental mechanism as a host-wide enforcement guarantee
without verifying the provider's inventory, deletion and resource-limit
contracts and accounting for all other writers. Provider-only or unaccounted
guests close the domain instead of being removed automatically. Runtime data,
credentials and actual deployment values do not belong in this example.

The charged vector describes guest CPU count, memory and root-disk limits.
Reserve a separate margin for hypervisor/control-plane overhead and all work
outside this controller; these probes do not measure total host consumption.
`jobs` bounds charged runner execution slots, not GitHub's queued or assigned
requests. The mode makes no pre-acquisition scheduling promise.

Use a direct, non-clustered Incus 6.0 endpoint with an exact pinned TLS leaf;
do not put a load balancer or independent backends sharing a certificate behind
it. The persisted identity binds that leaf, endpoint and project. Retire all
outstanding allocations before changing the backend identity. Unix sockets,
CA-only trust and unsupported server families remain closed in this mode.
