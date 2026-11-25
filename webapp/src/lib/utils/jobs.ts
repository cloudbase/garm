import type { Job, ScaleSetJob } from '$lib/api/generated/api.js';

// One shape for the two kinds of job GARM tracks, so the queue and runner
// views can treat them alike. Webhook jobs (handled by pools) are used as
// they come. Scale set jobs live in their own table with their own field
// names and are converted with fromScaleSetJob().
export type QueueJob = Job & {
	// GARM ID of the scale set that reported the job. Scale set jobs only.
	scale_set_id?: number;
	// GitHub's ID for the job on the scale set message queue. Scale set jobs only.
	scaleset_job_id?: string;
};

// Scale set jobs and webhook jobs are numbered independently. Scale set jobs
// get negative IDs here so the two cannot collide in lists keyed by id.
export function scaleSetJobKey(id: number): number {
	return -id;
}

export function fromScaleSetJob(job: ScaleSetJob): QueueJob {
	return {
		id: scaleSetJobKey(job.id ?? 0),
		scaleset_job_id: job.scaleset_job_id,
		scale_set_id: job.scaleset_id,
		name: job.name,
		status: job.status,
		conclusion: job.result,
		labels: job.request_labels,
		repository_name: job.repository_name,
		repository_owner: job.repository_owner,
		runner_name: job.runner_name,
		run_id: job.workflow_run_id,
		workflow_run_url: job.workflow_run_url,
		// "Waiting for" durations count from when GitHub queued the job.
		created_at: job.queue_time || job.created_at,
		updated_at: job.updated_at
	};
}

// The payload of a "scaleset_job" websocket event, converted to what a "job"
// event handler expects. Delete events may carry the bare ID.
export function scaleSetJobEventPayload(operation: string, payload: any): any {
	if (operation === 'delete') {
		return { id: scaleSetJobKey(payload?.id ?? payload) };
	}
	return fromScaleSetJob(payload as ScaleSetJob);
}

// Link to the job on GitHub. Webhook jobs carry the numeric GitHub job ID,
// allowing a direct /job/<id> link; scale set messages only carry a GUID,
// so those link to the workflow run page.
export function jobUrl(job: QueueJob): string {
	let runUrl = job.workflow_run_url;
	if (!runUrl && job.run_id && job.repository_owner && job.repository_name) {
		runUrl = `https://github.com/${job.repository_owner}/${job.repository_name}/actions/runs/${job.run_id}`;
	}
	if (!runUrl) return '';
	if (job.workflow_job_id) {
		return `${runUrl}/job/${job.workflow_job_id}`;
	}
	return runUrl;
}

// Map runner name -> the most relevant job for that runner: an in-progress
// job wins over queued/completed ones; ties go to the most recently updated.
export function buildJobsByRunner(jobs: QueueJob[]): Map<string, QueueJob> {
	const rank = (j: QueueJob) => (j.status === 'in_progress' ? 2 : j.status === 'queued' ? 1 : 0);
	const byRunner = new Map<string, QueueJob>();
	for (const job of jobs) {
		if (!job.runner_name) continue;
		const existing = byRunner.get(job.runner_name);
		if (
			!existing ||
			rank(job) > rank(existing) ||
			(rank(job) === rank(existing) &&
				new Date(job.updated_at || job.created_at || 0).getTime() >
					new Date(existing.updated_at || existing.created_at || 0).getTime())
		) {
			byRunner.set(job.runner_name, job);
		}
	}
	return byRunner;
}
