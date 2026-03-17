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
	type Template,
	type CreateTemplateParams,
	type UpdateTemplateParams,
	type RestoreTemplateRequest,
	type CreateRepoParams,
	type CreateOrgParams,
	type CreateEnterpriseParams,
	type CreatePoolParams,
	type CreateScaleSetParams,
	type UpdateEntityParams,
	type UpdatePoolParams,
	type LoginRequest,
	type LoginResponse,
	type FileObject,
	type FileObjectPaginatedResponse,
	type UpdateFileObjectParams,
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
	Template,
	CreateTemplateParams,
	UpdateTemplateParams,
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
	FileObject,
	FileObjectPaginatedResponse,
	UpdateFileObjectParams,
};

// Legacy APIError type for backward compatibility
export interface APIError {
	error: string;
	details?: string;
}

// Create a singleton instance
export const garmApi = new GeneratedGarmApiClient();
