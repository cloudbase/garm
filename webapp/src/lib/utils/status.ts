/**
 * Unified status formatting and styling utilities
 * Provides consistent status display and color coding across all pages
 */

/**
 * Formats status text for display by replacing underscores with spaces
 * and converting to proper case
 */
export function formatStatusText(status: string): string {
	if (!status) return '';
	return status.replace(/_/g, ' ').toLowerCase()
		.split(' ')
		.map(word => word.charAt(0).toUpperCase() + word.slice(1))
		.join(' ');
}

/**
 * Returns Tailwind CSS classes for status badges based on industry-standard color conventions:
 * - Green: Successfully running/active states
 * - Blue: Idle/ready states  
 * - Yellow/Amber: Warning or transitional states
 * - Purple: Creating/building states
 * - Orange: Deletion/termination in progress
 * - Red: Failed/error states
 * - Gray: Unknown/pending states
 */
export function getStatusBadgeClass(status: string): string {
	if (!status) {
		return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-500/10 dark:text-gray-400 dark:ring-gray-500/20';
	}
	
	const normalizedStatus = status.toLowerCase();
	
	switch (normalizedStatus) {
		// Successfully running states - Green
		case 'running':
		case 'online':
			return 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-500/10 dark:text-green-400 dark:ring-green-500/20';
		
		// Idle/ready states - Blue
		case 'idle':
		case 'stopped':
			return 'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-500/10 dark:text-blue-400 dark:ring-blue-500/20';
		
		// Active/working states - Yellow
		case 'active':
			return 'bg-yellow-50 text-yellow-700 ring-yellow-600/20 dark:bg-yellow-500/10 dark:text-yellow-400 dark:ring-yellow-500/20';
		
		// Creating/building states - Purple with pulse animation
		case 'creating':
		case 'installing':
		case 'pending_create':
		case 'provisioning':
			return 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-500/10 dark:text-purple-400 dark:ring-purple-500/20 animate-pulse';
		
		// Deletion/termination states - Orange with pulse animation
		case 'deleting':
		case 'terminating':
		case 'pending_delete':
		case 'destroying':
			return 'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-500/10 dark:text-orange-400 dark:ring-orange-500/20 animate-pulse';
		
		// Failed/error states - Red
		case 'failed':
		case 'error':
		case 'terminated':
		case 'offline':
			return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-500/10 dark:text-red-400 dark:ring-red-500/20';
		
		// General pending states - Gray with pulse animation
		case 'pending':
		case 'unknown':
			return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-500/10 dark:text-gray-400 dark:ring-gray-500/20 animate-pulse';
		
		// Default - Gray
		default:
			return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-500/10 dark:text-gray-400 dark:ring-gray-500/20';
	}
}

/**
 * Combined utility that returns both formatted text and CSS classes
 */
export function getFormattedStatus(status: string): { text: string; classes: string } {
	return {
		text: formatStatusText(status),
		classes: getStatusBadgeClass(status)
	};
}