/**
 * Read a file as text and return its content base64-encoded via callback.
 * Used for CA certificates, private keys, etc.
 */
export function readFileAsBase64(file: File, onResult: (base64: string) => void): void {
	const reader = new FileReader();
	reader.onload = (e) => {
		const content = e.target?.result as string;
		onResult(btoa(content));
	};
	reader.readAsText(file);
}

/**
 * Handle a file input change event. Reads the selected file as base64.
 * Calls onClear if no file is selected.
 */
export function handleFileInputAsBase64(
	event: Event,
	onResult: (base64: string, fileName: string) => void,
	onClear?: () => void
): void {
	const target = event.target as HTMLInputElement;
	const file = target.files?.[0];
	if (!file) {
		onClear?.();
		return;
	}
	readFileAsBase64(file, (base64) => onResult(base64, file.name));
}
