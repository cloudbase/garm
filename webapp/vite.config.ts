import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	// Load env variables based on the current mode
	// Third param '' means load all variables, not just those prefixed with VITE_
	const env = loadEnv(mode, process.cwd(), '');

	console.log(env.VITE_GARM_API_URL);
	return {
		plugins: [sveltekit()],
		server: {
			proxy: {
				// Proxy API calls to GARM backend
				'/api': {
					target: env.VITE_GARM_API_URL,
					changeOrigin: true,
					ws: true,
					configure: (proxy, _options) => {
						proxy.on('error', (err, _req, _res) => {
							console.log('proxy error', err);
						});
						proxy.on('proxyReq', (proxyReq, req, _res) => {
							console.log('Sending Request to the Target:', req.method, req.url);
						});
						proxy.on('proxyRes', (proxyRes, req, _res) => {
							console.log('Received Response from the Target:', proxyRes.statusCode, req.url);
						});
		  			},
					secure: false
				}
			}
		}
	};
});

