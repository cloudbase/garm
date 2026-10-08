import type { Job } from '$lib/api/generated/api.js';

// Link to the job on GitHub. Webhook jobs carry the numeric GitHub job ID,
// allowing a direct /job/<id> link; scale set messages only carry a GUID,
// so those link to the workflow run page.
export function jobUrl(job: Job): string {
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
export function buildJobsByRunner(jobs: Job[]): Map<string, Job> {
	const rank = (j: Job) => (j.status === 'in_progress' ? 2 : j.status === 'queued' ? 1 : 0);
	const byRunner = new Map<string, Job>();
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
