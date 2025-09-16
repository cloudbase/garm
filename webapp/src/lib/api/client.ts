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

// GarmApiClient now extends/wraps the generated client
export class GarmApiClient extends GeneratedGarmApiClient {
	constructor(baseUrl: string = '') {
		super(baseUrl);
	}

	// All methods are inherited from GeneratedGarmApiClient
	// This class now acts as a simple wrapper for backward compatibility
	
	// Explicitly expose template methods for TypeScript
	declare listTemplates: (osType?: string, partialName?: string, forgeType?: string) => Promise<Template[]>;
	declare getTemplate: (id: number) => Promise<Template>;
	declare createTemplate: (params: CreateTemplateParams) => Promise<Template>;
	declare updateTemplate: (id: number, params: UpdateTemplateParams) => Promise<Template>;
	declare deleteTemplate: (id: number) => Promise<void>;
	declare restoreTemplates: (params: RestoreTemplateRequest) => Promise<void>;

	// Explicitly expose file object methods for TypeScript
	declare listFileObjects: (tags?: string, page?: number, pageSize?: number) => Promise<FileObjectPaginatedResponse>;
	declare getFileObject: (objectID: string) => Promise<FileObject>;
	declare updateFileObject: (objectID: string, params: UpdateFileObjectParams) => Promise<FileObject>;
	declare deleteFileObject: (objectID: string) => Promise<void>;
}

// Create a singleton instance
export const garmApi = new GarmApiClient();
