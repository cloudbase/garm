import { describe, it, expect } from 'vitest';
import { fromScaleSetJob, scaleSetJobEventPayload, scaleSetJobKey, jobUrl } from './jobs.js';

describe('fromScaleSetJob', () => {
	const scaleSetJob = {
		id: 42,
		scaleset_job_id: 'guid-1',
		scaleset_id: 5,
		name: 'build',
		status: 'in_progress',
		result: '',
		request_labels: ['self-hosted', 'small'],
		repository_owner: 'acme',
		repository_name: 'widgets',
		runner_name: 'small-abc',
		workflow_run_id: 1234,
		workflow_run_url: 'https://github.com/acme/widgets/actions/runs/1234',
		queue_time: '2026-01-01T00:00:00Z',
		created_at: '2026-01-01T00:00:05Z',
		updated_at: '2026-01-01T00:01:00Z'
	};

	it('maps scale set job fields onto the shared job shape', () => {
		expect(fromScaleSetJob(scaleSetJob)).toEqual({
			id: -42,
			scaleset_job_id: 'guid-1',
			scale_set_id: 5,
			name: 'build',
			status: 'in_progress',
			conclusion: '',
			labels: ['self-hosted', 'small'],
			repository_owner: 'acme',
			repository_name: 'widgets',
			runner_name: 'small-abc',
			run_id: 1234,
			workflow_run_url: 'https://github.com/acme/widgets/actions/runs/1234',
			created_at: '2026-01-01T00:00:00Z',
			updated_at: '2026-01-01T00:01:00Z'
		});
	});

	it('falls back to the record creation time when GitHub sent no queue time', () => {
		const job = fromScaleSetJob({ ...scaleSetJob, queue_time: undefined });
		expect(job.created_at).toBe('2026-01-01T00:00:05Z');
	});

	it('keeps scale set job IDs apart from webhook job IDs', () => {
		expect(fromScaleSetJob({ id: 7 }).id).toBe(scaleSetJobKey(7));
		expect(scaleSetJobKey(7)).toBeLessThan(0);
	});

	it('links to the workflow run', () => {
		expect(jobUrl(fromScaleSetJob(scaleSetJob))).toBe(
			'https://github.com/acme/widgets/actions/runs/1234'
		);
	});
});

describe('scaleSetJobEventPayload', () => {
	it('converts create and update payloads', () => {
		const payload = scaleSetJobEventPayload('update', { id: 3, scaleset_id: 9, status: 'queued' });
		expect(payload).toMatchObject({ id: -3, scale_set_id: 9, status: 'queued' });
	});

	it('converts delete payloads given as an object or a bare ID', () => {
		expect(scaleSetJobEventPayload('delete', { id: 3 })).toEqual({ id: -3 });
		expect(scaleSetJobEventPayload('delete', 3)).toEqual({ id: -3 });
	});
});
