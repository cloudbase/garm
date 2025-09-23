<script lang="ts">
	import { onMount, onDestroy, createEventDispatcher } from 'svelte';
	import { EditorView, basicSetup } from 'codemirror';
	import { EditorState, StateEffect } from '@codemirror/state';
	import { oneDark } from '@codemirror/theme-one-dark';
	import { python } from '@codemirror/lang-python';
	import { StreamLanguage } from '@codemirror/language';
	import { shell } from '@codemirror/legacy-modes/mode/shell';
	import { autocompletion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
	import { themeStore } from '$lib/stores/theme.js';

	const dispatch = createEventDispatcher<{
		change: { value: string };
	}>();

	export let value: string = '';
	export let language: 'bash' | 'powershell' | 'python' | 'text' = 'text';
	export let readonly: boolean = false;
	export const placeholder: string = '';
	export let minHeight: string = '200px';
	export let autoDetect: boolean = true;
	export let enableTemplateCompletion: boolean = false;

	let editor: EditorView;
	let editorContainer: HTMLElement;
	
	// Theme management
	$: isDarkMode = $themeStore;

	// Get theme extensions
	function getThemeExtensions(darkMode: boolean) {
		return darkMode ? [oneDark] : [];
	}

	// Language extensions mapping
	function getLanguageExtension(lang: typeof language) {
		switch (lang) {
			case 'python':
				return python();
			case 'bash':
				return StreamLanguage.define(shell);
			case 'powershell':
				// Use shell highlighting for PowerShell for now
				return StreamLanguage.define(shell);
			case 'text':
			default:
				return [];
		}
	}

	// Auto-detect language from content
	function detectLanguage(content: string): typeof language {
		const lines = content.split('\n');
		const firstLine = lines[0]?.trim() || '';
		
		// Check shebang patterns
		if (firstLine.startsWith('#!/bin/bash') || firstLine.startsWith('#!/bin/sh')) {
			return 'bash';
		}
		if (firstLine.startsWith('#!/usr/bin/env pwsh') || firstLine.includes('#ps1_sysnative')) {
			return 'powershell';
		}
		if (firstLine.startsWith('#!/usr/bin/env python') || firstLine.startsWith('#!/usr/bin/python')) {
			return 'python';
		}
		
		// Check for PowerShell patterns
		if (content.includes('param(') || content.includes('Write-Host') || content.includes('$_')) {
			return 'powershell';
		}
		
		// Check for Python patterns
		if (content.includes('def ') || content.includes('import ') || content.includes('print(')) {
			return 'python';
		}
		
		// Check for bash patterns
		if (content.includes('echo ') || content.includes('export ') || content.includes('if [')) {
			return 'bash';
		}
		
		return 'text';
	}

	// Update language if auto-detection is enabled, content changes and language is text
	$: if (autoDetect && language === 'text' && value) {
		const detectedLang = detectLanguage(value);
		if (detectedLang !== 'text') {
			language = detectedLang;
		}
	}

	// Template variables from InstallRunnerParams struct
	const templateVariables = [
		{ label: 'RunnerName', type: 'string', detail: 'The name of the runner instance' },
		{ label: 'MetadataURL', type: 'string', detail: 'URL for accessing runner metadata service' },
		{ label: 'Token', type: 'string', detail: 'Authentication token for runner registration' },
		{ label: 'RepoURL', type: 'string', detail: 'Repository URL' },
		{ label: 'DownloadURL', type: 'string', detail: 'URL to download the runner package' },
		{ label: 'FileName', type: 'string', detail: 'Name of the downloaded runner file' },
		{ label: 'TempDir', type: 'string', detail: 'Temporary directory path' },
		{ label: 'RunnerDir', type: 'string', detail: 'Runner installation directory' },
		{ label: 'GitHubRunnerGroup', type: 'string', detail: 'GitHub runner group name' },
		{ label: 'Labels', type: '[]string', detail: 'Array of runner labels' },
		{ label: 'CACertBundle', type: '[]byte', detail: 'CA certificate bundle' },
		{ label: 'OSType', type: 'string', detail: 'Operating system type (linux/windows)' },
		{ label: 'OSArch', type: 'string', detail: 'Operating system architecture (amd64/arm64)' },
		{ label: 'Flavor', type: 'string', detail: 'Runner flavor/size specification' },
		{ label: 'Image', type: 'string', detail: 'Runner image name' },
		{ label: 'ImageOS', type: 'string', detail: 'Operating system of the image' },
		{ label: 'ImageOSVersion', type: 'string', detail: 'Version of the operating system' },
		{ label: 'PoolID', type: 'string', detail: 'Pool identifier' },
		{ label: 'ExtraSpecs', type: 'json.RawMessage', detail: 'Additional specifications as JSON' },
		{ label: 'JitConfigEnabled', type: 'bool', detail: 'Whether just-in-time configuration is enabled' },
		{ label: 'UserDataOptions', type: 'UserDataOptions', detail: 'User data configuration options' }
	];

	// Go template functions
	const templateFunctions = [
		{ label: 'print', type: 'function', detail: 'Print arguments to output' },
		{ label: 'printf', type: 'function', detail: 'Print formatted string' },
		{ label: 'println', type: 'function', detail: 'Print arguments with newline' },
		{ label: 'len', type: 'function', detail: 'Get length of array, slice, map, or string' },
		{ label: 'index', type: 'function', detail: 'Get element at index from array or slice' },
		{ label: 'slice', type: 'function', detail: 'Create slice from array' },
		{ label: 'range', type: 'function', detail: 'Iterate over array, slice, or map' },
		{ label: 'if', type: 'function', detail: 'Conditional execution' },
		{ label: 'else', type: 'function', detail: 'Alternative execution branch' },
		{ label: 'end', type: 'function', detail: 'End block statement' },
		{ label: 'with', type: 'function', detail: 'Change context to specified value' },
		{ label: 'template', type: 'function', detail: 'Include another template' },
		{ label: 'define', type: 'function', detail: 'Define a named template' },
		{ label: 'block', type: 'function', detail: 'Define a template block' }
	];

	// Template completion function
	function templateCompletion(context: CompletionContext): CompletionResult | null {
		if (!enableTemplateCompletion) return null;
		
		const line = context.state.doc.lineAt(context.pos);
		const lineText = line.text;
		const pos = context.pos - line.from;
		
		// Check if we're inside template delimiters {{}}
		const beforeCursor = lineText.slice(0, pos);
		const afterCursor = lineText.slice(pos);
		
		const templateStart = beforeCursor.lastIndexOf('{{');
		const templateEnd = afterCursor.indexOf('}}');
		
		// Only provide completions if we're inside {{}}
		if (templateStart === -1 || templateEnd === -1) {
			return null;
		}
		
		// Get the text between {{ and cursor position
		const templateContent = beforeCursor.slice(templateStart + 2);
		
		// Check if we're trying to complete a variable (starts with .)
		const dotMatch = templateContent.match(/\.(\w*)$/);
		if (dotMatch) {
			const prefix = dotMatch[1];
			const startPos = context.pos - prefix.length;
			
			return {
				from: startPos,
				options: templateVariables
					.filter(v => v.label.toLowerCase().startsWith(prefix.toLowerCase()))
					.map(v => ({
						label: v.label,
						type: v.type,
						detail: v.detail,
						info: `${v.type}: ${v.detail}`
					}))
			};
		}
		
		// Check if we're trying to complete a function or keyword
		const wordMatch = templateContent.match(/(\w+)$/);
		if (wordMatch) {
			const prefix = wordMatch[1];
			const startPos = context.pos - prefix.length;
			
			const allCompletions = [
				...templateFunctions.map(f => ({
					label: f.label,
					type: f.type,
					detail: f.detail,
					info: f.detail
				})),
				...templateVariables.map(v => ({
					label: `.${v.label}`,
					type: v.type,
					detail: v.detail,
					info: `${v.type}: ${v.detail}`
				}))
			];
			
			return {
				from: startPos,
				options: allCompletions
					.filter(c => c.label.toLowerCase().includes(prefix.toLowerCase()))
			};
		}
		
		// If we're at the start of template content, show all options
		if (templateContent.trim() === '') {
			return {
				from: context.pos,
				options: [
					...templateFunctions.map(f => ({
						label: f.label,
						type: f.type,
						detail: f.detail,
						info: f.detail
					})),
					...templateVariables.map(v => ({
						label: `.${v.label}`,
						type: v.type,
						detail: v.detail,
						info: `${v.type}: ${v.detail}`
					}))
				]
			};
		}
		
		return null;
	}

	function createEditor() {
		if (!editorContainer) return;

		const extensions = [
			basicSetup,
			...getThemeExtensions(isDarkMode),
			getLanguageExtension(language),
			EditorView.updateListener.of((update) => {
				if (update.docChanged) {
					const newValue = update.state.doc.toString();
					value = newValue;
					dispatch('change', { value: newValue });
				}
			}),
			EditorState.readOnly.of(readonly),
			EditorView.theme({
				'&': {
					minHeight: minHeight
				},
				'.cm-editor': {
					minHeight: minHeight
				},
				'.cm-scroller': {
					minHeight: minHeight
				}
			})
		];
		
		// Add template completion if enabled
		if (enableTemplateCompletion) {
			extensions.push(autocompletion({ override: [templateCompletion] }));
		}
		
		const state = EditorState.create({
			doc: value,
			extensions
		});

		editor = new EditorView({
			state,
			parent: editorContainer
		});
	}

	function updateEditor() {
		if (!editor) return;

		const currentValue = editor.state.doc.toString();
		if (currentValue !== value) {
			editor.dispatch({
				changes: {
					from: 0,
					to: currentValue.length,
					insert: value
				}
			});
		}

		// Update language extension
		const newExtensions = [
			basicSetup,
			...getThemeExtensions(isDarkMode),
			getLanguageExtension(language),
			EditorView.updateListener.of((update) => {
				if (update.docChanged) {
					const newValue = update.state.doc.toString();
					value = newValue;
					dispatch('change', { value: newValue });
				}
			}),
			EditorState.readOnly.of(readonly),
			EditorView.theme({
				'&': {
					minHeight: minHeight
				},
				'.cm-editor': {
					minHeight: minHeight
				},
				'.cm-scroller': {
					minHeight: minHeight
				}
			})
		];
		
		// Add template completion if enabled
		if (enableTemplateCompletion) {
			newExtensions.push(autocompletion({ override: [templateCompletion] }));
		}

		editor.dispatch({
			effects: StateEffect.reconfigure.of(newExtensions)
		});
	}

	onMount(() => {
		createEditor();
	});

	onDestroy(() => {
		if (editor) {
			editor.destroy();
		}
	});

	// React to prop changes
	$: if (editor && language) {
		updateEditor();
	}

	$: if (editor && value !== editor.state.doc.toString()) {
		updateEditor();
	}

	// React to theme changes
	$: if (editor && isDarkMode !== undefined) {
		updateEditor();
	}
</script>

<div bind:this={editorContainer} class="border border-gray-300 dark:border-gray-600 rounded-md overflow-hidden"></div>