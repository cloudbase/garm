import type { APIErrorResponse } from '$lib/api/generated/api';

/**
 * Extracts error message from API error response
 * @param error - The error object from API call
 * @returns Human-readable error message
 * 
 * @example
 * ```typescript
 * try {
 *   await garmApi.deletePool(poolId);
 * } catch (error) {
 *   const message = extractAPIError(error);
 *   // Will return "Pool deletion failed. Pool has active runners" if both error and details exist
 *   // Or just "Pool deletion failed" if only error exists
 *   // Or just "Pool has active runners" if only details exist
 *   toastStore.error('Delete Failed', message);
 * }
 * ```
 */
export function extractAPIError(error: any): string {
	// Default fallback message
	let errorMessage = 'An unexpected error occurred';

	// Try to extract APIErrorResponse from the error
	if (error && typeof error === 'object') {
		// Check if it's an axios error with response
		if ('response' in error && error.response && typeof error.response === 'object') {
			const response = error.response;
			
			// Check if response has data with APIErrorResponse structure
			if ('data' in response && response.data && typeof response.data === 'object') {
				const apiError = response.data as APIErrorResponse;
				
				// Build message from available fields
				const errorPart = apiError.error && apiError.error.trim() ? apiError.error : '';
				const detailsPart = apiError.details && apiError.details.trim() ? apiError.details : '';
				
				if (errorPart && detailsPart) {
					// Both available - combine them
					return `${errorPart}. ${detailsPart}`;
				} else if (errorPart) {
					// Only error available
					return errorPart;
				} else if (detailsPart) {
					// Only details available
					return detailsPart;
				}
			}
			
			// If no APIErrorResponse, try to get status-based message
			if ('status' in response) {
				const status = response.status;
				switch (status) {
					case 400:
						errorMessage = 'Bad request - please check your input';
						break;
					case 401:
						errorMessage = 'Unauthorized - please log in again';
						break;
					case 403:
						errorMessage = 'Access denied - insufficient permissions';
						break;
					case 404:
						errorMessage = 'Resource not found';
						break;
					case 409:
						errorMessage = 'Conflict - resource already exists or is in use';
						break;
					case 422:
						errorMessage = 'Validation failed - please check your input';
						break;
					case 500:
						errorMessage = 'Internal server error - please try again later';
						break;
					default:
						errorMessage = `Request failed with status ${status}`;
				}
			}
		}
		// Check if it's a direct Error object with a meaningful message
		else if (error instanceof Error && error.message && !error.message.includes('status code')) {
			return error.message;
		}
	}

	return errorMessage;
}