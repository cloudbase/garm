/**
 * Format file size in bytes to human-readable format
 */
export function formatFileSize(bytes: number): string {
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let size = bytes;
	let unitIndex = 0;

	while (size >= 1024 && unitIndex < units.length - 1) {
		size /= 1024;
		unitIndex++;
	}

	return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

/**
 * Format datetime string to readable format
 */
export function formatDateTime(dateString: string | undefined): string {
	if (!dateString) return 'N/A';

	try {
		const date = new Date(dateString);
		return date.toLocaleString();
	} catch {
		return dateString;
	}
}
