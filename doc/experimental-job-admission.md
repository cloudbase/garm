# Experimental job admission

This mode is an experimental extension of [VM resource admission](experimental-vm-admission.md). Its initial unit is a durable one-slot escrow for each enrolled scale set. Each escrow reserves the configured VM resource vector and one job slot from the shared domain **before** the scale set opens its message session or advertises positive capacity. Independent sets can compete for the shared budget; a set without an escrow cannot advertise a positive capacity. The escrow remains owned by its set and remains charged across long-poll timeouts, lost replies, session restarts and controller restarts. Polls reuse the same escrow rather than reserving one slot per request. If a
positive poll's outcome is lost, or the controller restarts with that poll
still pending, new positive admission remains parked. Zero-capacity polling
continues for lifecycle processing; there is no automatic reset or TTL-based
recovery for the unresolved poll.

Select `protocol = "job-capacity-experimental"` and set
`fresh_scale_set_confirmed = true` in the admission configuration. Set the
runner vector's `vms` and `jobs` to `1`; each escrow consumes one of each.
This confirmation is an
operator affirmation, not a discovery mechanism. Enroll only an explicitly
approved fresh scale set without previous unaccounted jobs, runners or capacity advertisements. Initial session statistics must be reconciled before advertising capacity. The adapter must not infer a clean historical handover merely from an empty current message. The provider, profile, resource, external-workload envelope and inventory requirements of VM resource admission continue to apply. Warm or min-idle runners are outside this first mode.

For a healthy, enabled and active set, the advertised `maxCapacity` is at most one. A closed health gate or administrative drain advertises zero while the listener continues processing lifecycle messages for work already known. An earlier positive advertisement or in-flight poll might still have effects after a later zero: zero is a request to stop offering new capacity, not a verified acknowledgment that GitHub has revoked prior offers. The escrow remains charged while that uncertainty exists. Neither a 202 response, timeout, message acknowledgment, session deletion, health expiration nor an elapsed TTL releases it. The mode does not automatically transfer an escrow to another set or promise cross-set failover.

An assigned job consumes the set's existing escrow; creating its runner binds that same VM allocation rather than charging the resource vector twice. A later job can be assigned while the previous runner is deleting. In that case its work remains queued under the same escrow until the old guest's cleanup is confirmed; a second VM must not be created using the same credit. Job lifecycle completion and provider-resource cleanup are distinct observations. A lost or ambiguous assignment, acquisition, create or delete response retains the corresponding uncertainty and resource charge until authoritative reconciliation; it is not proof of failure or permission to retry a non-idempotent operation.

Reducing a domain budget below its aggregate held charges blocks new positive
polls, acquisitions and VM creation. Existing guests are preserved and escrows
remain charged. Restoring the budget removes this budget block; health, drain,
generation and unresolved-operation checks still apply. The mode does not
automatically retire or redistribute permanent escrows to fit a smaller budget.

The listener must process assigned, started and completed events even while new work is closed. Duplicate and mixed messages must be handled durably and idempotently; do not advance the message cursor past unretained selected work. An unselected available notification need not be acquired; acknowledging it does not establish that the offer will be redelivered or routed elsewhere. Session and message statistics are required to account for assigned jobs across restart, including messages omitted from a truncated batch. If request identity, attempt correlation, or state cannot be reconciled, the affected slot remains closed. Draining preserves active work and persisted operator intent; completion requires proven cleanup and resolution of outstanding assignments, not merely sending capacity zero.

[GitHub's scale-set client guidance](https://github.com/actions/scaleset#autoscaling) describes `TotalAssignedJobs` as including waiting and running jobs, notes that individual message batches can be truncated, and describes the `maxCapacity` polling header. The [current listener interface](https://github.com/actions/scaleset/blob/main/listener/listener.go) also instructs scalers to acquire desired `JobAvailable` requests and process replay safely. These describe a public-preview client and do not establish every server version's acquisition, partial-reply, redelivery or revocation guarantees. The [ARC drain example](https://docs.github.com/en/actions/how-tos/manage-runners/use-actions-runner-controller/deploy-runner-scale-sets#example-jobs-queue-draining) specifies that zero prevents new runner pods; it does not prove immediate cancellation of prior capacity advertisements. Actual queue visibility, selection, recovery, and failover require separate disposable integration evidence.

Do not roll back migration `0013_admission_jobs`, downgrade to a binary that
does not enforce this mode, or remove the job-capacity configuration while
escrows or outstanding assignments remain. Such a change would discard or bypass
the durable accounting. Sending zero and deleting a message session do not
prove that a previous advertisement or assignment has been retired. A safe
handover requires authoritative evidence for the relevant GitHub protocol and
provider cleanup; until then retain the escrow and keep the mode enrolled.

This is an experimental implementation for review, not evidence that the
GitHub service has honored the requested ceiling or drain. In particular, an
unresolved poll, missing event identity, or ambiguous acquired request may
leave a set parked until the protocol outcome can be established. The mode does
not provide an automatic way to reconcile every such condition.
