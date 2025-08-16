import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ url }) => {
	// For now, we'll handle auth redirect in the component
	// In a real app, you might check auth state here
	
	return {
		url: url.pathname
	};
};

export const prerender = false;
export const ssr = false;