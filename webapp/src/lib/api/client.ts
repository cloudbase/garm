// Importing from the generated client wrapper
import { 
	GeneratedGarmApiClient, 
	type Repository,
	type Organization,
	type Enterprise,
	type Endpoint,
	type Pool,
	type ScaleSet,
	type Instance,
	type ForgeCredentials,
	type Provider,
	type ControllerInfo,
	type CreateRepoParams,
	type CreateOrgParams,
	type CreateEnterpriseParams,
	type CreatePoolParams,
	type CreateScaleSetParams,
	type UpdateEntityParams,
	type UpdatePoolParams,
	type LoginRequest,
	type LoginResponse,
} from './generated-client.js';

// Import endpoint and credentials types directly
import type {
	CreateGithubEndpointParams as CreateEndpointParams,
	UpdateGithubEndpointParams as UpdateEndpointParams,
	CreateGithubCredentialsParams as CreateCredentialsParams,
	UpdateGithubCredentialsParams as UpdateCredentialsParams,
} from './generated/api';

// Re-export types for compatibility
export type {
	Repository,
	Organization,
	Enterprise,
	Endpoint,
	Pool,
	ScaleSet,
	Instance,
	ForgeCredentials,
	Provider,
	ControllerInfo,
	CreateRepoParams,
	CreateOrgParams,
	CreateEnterpriseParams,
	CreateEndpointParams,
	UpdateEndpointParams,
	CreateCredentialsParams,
	UpdateCredentialsParams,
	CreatePoolParams,
	CreateScaleSetParams,
	UpdateEntityParams,
	UpdatePoolParams,
	LoginRequest,
	LoginResponse,
};

// Legacy APIError type for backward compatibility
export interface APIError {
	error: string;
	details?: string;
}

// GarmApiClient now extends/wraps the generated client
export class GarmApiClient extends GeneratedGarmApiClient {
	constructor(baseUrl: string = '') {
		super(baseUrl);
	}

	// All methods are inherited from GeneratedGarmApiClient
	// This class now acts as a simple wrapper for backward compatibility
}

// Create a singleton instance
export const garmApi = new GarmApiClient();
