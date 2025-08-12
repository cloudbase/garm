// Generated API Client Wrapper for GARM
// This wraps the auto-generated OpenAPI client to match our existing interface

import {
  LoginApi,
  ControllerInfoApi,
  ControllerApi,
  EndpointsApi,
  CredentialsApi,
  RepositoriesApi,
  OrganizationsApi,
  EnterprisesApi,
  PoolsApi,
  ScalesetsApi,
  InstancesApi,
  ProvidersApi,
  FirstRunApi,
  HooksApi,
  type Repository,
  type Organization,
  type Enterprise,
  type ForgeEndpoint,
  type Pool,
  type ScaleSet,
  type Instance,
  type ForgeCredentials,
  type Provider,
  type ControllerInfo,
  type CreateRepoParams,
  type CreateOrgParams,
  type CreateEnterpriseParams,
  type CreateGithubEndpointParams,
  type CreateGiteaEndpointParams,
  type UpdateGithubEndpointParams,
  type UpdateGiteaEndpointParams,
  type CreateGithubCredentialsParams,
  type CreateGiteaCredentialsParams,
  type UpdateGithubCredentialsParams,
  type UpdateGiteaCredentialsParams,
  type CreatePoolParams,
  type CreateScaleSetParams,
  type UpdateEntityParams,
  type UpdatePoolParams,
  type PasswordLoginParams,
  type JWTResponse,
  type NewUserParams,
  type User,
  type UpdateControllerParams,
  type HookInfo,
  Configuration
} from './generated/index';

// Re-export types for compatibility
export type {
  Repository,
  Organization,
  Enterprise,
  ForgeEndpoint as Endpoint,
  Pool,
  ScaleSet,
  Instance,
  ForgeCredentials,
  Provider,
  ControllerInfo,
  CreateRepoParams,
  CreateOrgParams,
  CreateEnterpriseParams,
  CreateGithubEndpointParams as CreateEndpointParams,
  UpdateGithubEndpointParams as UpdateEndpointParams,
  CreateGithubCredentialsParams as CreateCredentialsParams,
  UpdateGithubCredentialsParams as UpdateCredentialsParams,
  CreatePoolParams,
  CreateScaleSetParams,
  UpdateEntityParams,
  UpdatePoolParams,
  PasswordLoginParams,
  JWTResponse,
  NewUserParams,
  User,
  UpdateControllerParams,
};

// Define common request types for compatibility
export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export class GeneratedGarmApiClient {
  private baseUrl: string;
  private token?: string;
  private config: Configuration;
  
  // Check if we're in development mode (cross-origin setup)
  private isDevelopmentMode(): boolean {
    if (typeof window === 'undefined') return false;
    // Development mode: either VITE_GARM_API_URL is set OR we detect cross-origin
    return !!(import.meta.env.VITE_GARM_API_URL) || window.location.port === '5173';
  }

  // Generated API client instances
  private loginApi: LoginApi;
  private controllerInfoApi: ControllerInfoApi;
  private controllerApi: ControllerApi;
  private endpointsApi: EndpointsApi;
  private credentialsApi: CredentialsApi;
  private repositoriesApi: RepositoriesApi;
  private organizationsApi: OrganizationsApi;
  private enterprisesApi: EnterprisesApi;
  private poolsApi: PoolsApi;
  private scaleSetsApi: ScalesetsApi;
  private instancesApi: InstancesApi;
  private providersApi: ProvidersApi;
  private firstRunApi: FirstRunApi;
  private hooksApi: HooksApi;

  constructor(baseUrl: string = '') {
    this.baseUrl = baseUrl || window.location.origin;
    
    // Create configuration for the generated client
    const isDevMode = this.isDevelopmentMode();
    this.config = new Configuration({
      basePath: `${this.baseUrl}/api/v1`,
      accessToken: () => this.token || '',
      baseOptions: {
        // In development mode, don't send cookies (use Bearer token only)
        // In production mode, include cookies for authentication
        withCredentials: !isDevMode,
      },
    });

    // Initialize generated API clients
    this.loginApi = new LoginApi(this.config);
    this.controllerInfoApi = new ControllerInfoApi(this.config);
    this.controllerApi = new ControllerApi(this.config);
    this.endpointsApi = new EndpointsApi(this.config);
    this.credentialsApi = new CredentialsApi(this.config);
    this.repositoriesApi = new RepositoriesApi(this.config);
    this.organizationsApi = new OrganizationsApi(this.config);
    this.enterprisesApi = new EnterprisesApi(this.config);
    this.poolsApi = new PoolsApi(this.config);
    this.scaleSetsApi = new ScalesetsApi(this.config);
    this.instancesApi = new InstancesApi(this.config);
    this.providersApi = new ProvidersApi(this.config);
    this.firstRunApi = new FirstRunApi(this.config);
    this.hooksApi = new HooksApi(this.config);
  }

  // Set authentication token
  setToken(token: string) {
    this.token = token;
    
    // Update configuration for all clients
    const isDevMode = this.isDevelopmentMode();
    this.config = new Configuration({
      basePath: `${this.baseUrl}/api/v1`,
      accessToken: () => token,
      baseOptions: {
        // In development mode, don't send cookies (use Bearer token only)
        // In production mode, include cookies for authentication
        withCredentials: !isDevMode,
      },
    });

    // Recreate all API instances with new config
    this.loginApi = new LoginApi(this.config);
    this.controllerInfoApi = new ControllerInfoApi(this.config);
    this.controllerApi = new ControllerApi(this.config);
    this.endpointsApi = new EndpointsApi(this.config);
    this.credentialsApi = new CredentialsApi(this.config);
    this.repositoriesApi = new RepositoriesApi(this.config);
    this.organizationsApi = new OrganizationsApi(this.config);
    this.enterprisesApi = new EnterprisesApi(this.config);
    this.poolsApi = new PoolsApi(this.config);
    this.scaleSetsApi = new ScalesetsApi(this.config);
    this.instancesApi = new InstancesApi(this.config);
    this.providersApi = new ProvidersApi(this.config);
    this.firstRunApi = new FirstRunApi(this.config);
    this.hooksApi = new HooksApi(this.config);
  }

  // Authentication
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    const params: PasswordLoginParams = {
      username: credentials.username,
      password: credentials.password,
    };
    const response = await this.loginApi.login(params);
    const token = response.data.token;
    if (token) {
      this.setToken(token);
      return { token };
    }
    throw new Error('Login failed');
  }

  async getControllerInfo(): Promise<ControllerInfo> {
    const response = await this.controllerInfoApi.controllerInfo();
    return response.data;
  }

  // GitHub Endpoints
  async listGithubEndpoints(): Promise<ForgeEndpoint[]> {
    const response = await this.endpointsApi.listGithubEndpoints();
    return response.data || [];
  }

  async getGithubEndpoint(name: string): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.getGithubEndpoint(name);
    return response.data;
  }

  async createGithubEndpoint(params: CreateGithubEndpointParams): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.createGithubEndpoint(params);
    return response.data;
  }

  async updateGithubEndpoint(name: string, params: UpdateGithubEndpointParams): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.updateGithubEndpoint(name, params);
    return response.data;
  }

  async deleteGithubEndpoint(name: string): Promise<void> {
    await this.endpointsApi.deleteGithubEndpoint(name);
  }

  // Gitea Endpoints
  async listGiteaEndpoints(): Promise<ForgeEndpoint[]> {
    const response = await this.endpointsApi.listGiteaEndpoints();
    return response.data || [];
  }

  async getGiteaEndpoint(name: string): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.getGiteaEndpoint(name);
    return response.data;
  }

  async createGiteaEndpoint(params: CreateGiteaEndpointParams): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.createGiteaEndpoint(params);
    return response.data;
  }

  async updateGiteaEndpoint(name: string, params: UpdateGiteaEndpointParams): Promise<ForgeEndpoint> {
    const response = await this.endpointsApi.updateGiteaEndpoint(name, params);
    return response.data;
  }

  async deleteGiteaEndpoint(name: string): Promise<void> {
    await this.endpointsApi.deleteGiteaEndpoint(name);
  }

  // Combined Endpoints helper
  async listAllEndpoints(): Promise<ForgeEndpoint[]> {
    const [githubEndpoints, giteaEndpoints] = await Promise.all([
      this.listGithubEndpoints().catch(() => []),
      this.listGiteaEndpoints().catch(() => [])
    ]);

    return [
      ...githubEndpoints.map(ep => ({ ...ep, endpoint_type: 'github' as const })),
      ...giteaEndpoints.map(ep => ({ ...ep, endpoint_type: 'gitea' as const }))
    ];
  }

  // GitHub Credentials
  async listGithubCredentials(): Promise<ForgeCredentials[]> {
    const response = await this.credentialsApi.listCredentials();
    return response.data || [];
  }

  async getGithubCredentials(id: number): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.getCredentials(id);
    return response.data;
  }

  async createGithubCredentials(params: CreateGithubCredentialsParams): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.createCredentials(params);
    return response.data;
  }

  async updateGithubCredentials(id: number, params: UpdateGithubCredentialsParams): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.updateCredentials(id, params);
    return response.data;
  }

  async deleteGithubCredentials(id: number): Promise<void> {
    await this.credentialsApi.deleteCredentials(id);
  }

  // Gitea Credentials
  async listGiteaCredentials(): Promise<ForgeCredentials[]> {
    const response = await this.credentialsApi.listGiteaCredentials();
    return response.data || [];
  }

  async getGiteaCredentials(id: number): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.getGiteaCredentials(id);
    return response.data;
  }

  async createGiteaCredentials(params: CreateGiteaCredentialsParams): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.createGiteaCredentials(params);
    return response.data;
  }

  async updateGiteaCredentials(id: number, params: UpdateGiteaCredentialsParams): Promise<ForgeCredentials> {
    const response = await this.credentialsApi.updateGiteaCredentials(id, params);
    return response.data;
  }

  async deleteGiteaCredentials(id: number): Promise<void> {
    await this.credentialsApi.deleteGiteaCredentials(id);
  }

  // Combined Credentials helper
  async listAllCredentials(): Promise<ForgeCredentials[]> {
    const [githubCredentials, giteaCredentials] = await Promise.all([
      this.listGithubCredentials().catch(() => []),
      this.listGiteaCredentials().catch(() => [])
    ]);

    return [...githubCredentials, ...giteaCredentials];
  }

  // Repositories
  async installRepositoryWebhook(repoId: string, params: any = {}): Promise<void> {
    await this.repositoriesApi.installRepoWebhook(repoId, params);
  }

  async uninstallRepositoryWebhook(repoId: string): Promise<void> {
    await this.hooksApi.uninstallRepoWebhook(repoId);
  }

  async getRepositoryWebhookInfo(repoId: string): Promise<HookInfo> {
    const response = await this.hooksApi.getRepoWebhookInfo(repoId);
    return response.data;
  }
  async listRepositories(): Promise<Repository[]> {
    const response = await this.repositoriesApi.listRepos();
    return response.data || [];
  }

  async getRepository(id: string): Promise<Repository> {
    const response = await this.repositoriesApi.getRepo(id);
    return response.data;
  }

  async createRepository(params: CreateRepoParams): Promise<Repository> {
    const response = await this.repositoriesApi.createRepo(params);
    return response.data;
  }

  async updateRepository(id: string, params: UpdateEntityParams): Promise<Repository> {
    const response = await this.repositoriesApi.updateRepo(id, params);
    return response.data;
  }

  async deleteRepository(id: string): Promise<void> {
    await this.repositoriesApi.deleteRepo(id);
  }

  async installRepoWebhook(id: string): Promise<void> {
    await this.repositoriesApi.installRepoWebhook(id, {});
  }

  async listRepositoryPools(id: string): Promise<Pool[]> {
    const response = await this.repositoriesApi.listRepoPools(id);
    return response.data || [];
  }

  async listRepositoryInstances(id: string): Promise<Instance[]> {
    const response = await this.repositoriesApi.listRepoInstances(id);
    return response.data || [];
  }

  async createRepositoryPool(id: string, params: CreatePoolParams): Promise<Pool> {
    const response = await this.repositoriesApi.createRepoPool(id, params);
    return response.data;
  }

  // Organizations
  async installOrganizationWebhook(orgId: string, params: any = {}): Promise<void> {
    await this.organizationsApi.installOrgWebhook(orgId, params);
  }

  async uninstallOrganizationWebhook(orgId: string): Promise<void> {
    await this.hooksApi.uninstallOrgWebhook(orgId);
  }

  async getOrganizationWebhookInfo(orgId: string): Promise<HookInfo> {
    const response = await this.hooksApi.getOrgWebhookInfo(orgId);
    return response.data;
  }
  async listOrganizations(): Promise<Organization[]> {
    const response = await this.organizationsApi.listOrgs();
    return response.data || [];
  }

  async getOrganization(id: string): Promise<Organization> {
    const response = await this.organizationsApi.getOrg(id);
    return response.data;
  }

  async createOrganization(params: CreateOrgParams): Promise<Organization> {
    const response = await this.organizationsApi.createOrg(params);
    return response.data;
  }

  async updateOrganization(id: string, params: UpdateEntityParams): Promise<Organization> {
    const response = await this.organizationsApi.updateOrg(id, params);
    return response.data;
  }

  async deleteOrganization(id: string): Promise<void> {
    await this.organizationsApi.deleteOrg(id);
  }

  async listOrganizationPools(id: string): Promise<Pool[]> {
    const response = await this.organizationsApi.listOrgPools(id);
    return response.data || [];
  }

  async listOrganizationInstances(id: string): Promise<Instance[]> {
    const response = await this.organizationsApi.listOrgInstances(id);
    return response.data || [];
  }

  async createOrganizationPool(id: string, params: CreatePoolParams): Promise<Pool> {
    const response = await this.organizationsApi.createOrgPool(id, params);
    return response.data;
  }

  // Enterprises
  async listEnterprises(): Promise<Enterprise[]> {
    const response = await this.enterprisesApi.listEnterprises();
    return response.data || [];
  }

  async getEnterprise(id: string): Promise<Enterprise> {
    const response = await this.enterprisesApi.getEnterprise(id);
    return response.data;
  }

  async createEnterprise(params: CreateEnterpriseParams): Promise<Enterprise> {
    const response = await this.enterprisesApi.createEnterprise(params);
    return response.data;
  }

  async updateEnterprise(id: string, params: UpdateEntityParams): Promise<Enterprise> {
    const response = await this.enterprisesApi.updateEnterprise(id, params);
    return response.data;
  }

  async deleteEnterprise(id: string): Promise<void> {
    await this.enterprisesApi.deleteEnterprise(id);
  }

  async listEnterprisePools(id: string): Promise<Pool[]> {
    const response = await this.enterprisesApi.listEnterprisePools(id);
    return response.data || [];
  }

  async listEnterpriseInstances(id: string): Promise<Instance[]> {
    const response = await this.enterprisesApi.listEnterpriseInstances(id);
    return response.data || [];
  }

  async createEnterprisePool(id: string, params: CreatePoolParams): Promise<Pool> {
    const response = await this.enterprisesApi.createEnterprisePool(id, params);
    return response.data;
  }

  // Scale sets for repositories, organizations, and enterprises
  async createRepositoryScaleSet(id: string, params: CreateScaleSetParams): Promise<ScaleSet> {
    const response = await this.repositoriesApi.createRepoScaleSet(id, params);
    return response.data;
  }

  async listRepositoryScaleSets(id: string): Promise<ScaleSet[]> {
    const response = await this.repositoriesApi.listRepoScaleSets(id);
    return response.data || [];
  }

  async createOrganizationScaleSet(id: string, params: CreateScaleSetParams): Promise<ScaleSet> {
    const response = await this.organizationsApi.createOrgScaleSet(id, params);
    return response.data;
  }

  async listOrganizationScaleSets(id: string): Promise<ScaleSet[]> {
    const response = await this.organizationsApi.listOrgScaleSets(id);
    return response.data || [];
  }

  async createEnterpriseScaleSet(id: string, params: CreateScaleSetParams): Promise<ScaleSet> {
    const response = await this.enterprisesApi.createEnterpriseScaleSet(id, params);
    return response.data;
  }

  async listEnterpriseScaleSets(id: string): Promise<ScaleSet[]> {
    const response = await this.enterprisesApi.listEnterpriseScaleSets(id);
    return response.data || [];
  }

  // Pools
  async listPools(): Promise<Pool[]> {
    const response = await this.poolsApi.listPools();
    return response.data || [];
  }

  async listAllPools(): Promise<Pool[]> {
    return this.listPools();
  }

  async getPool(id: string): Promise<Pool> {
    const response = await this.poolsApi.getPool(id);
    return response.data;
  }

  async updatePool(id: string, params: UpdatePoolParams): Promise<Pool> {
    const response = await this.poolsApi.updatePool(id, params);
    return response.data;
  }

  async deletePool(id: string): Promise<void> {
    await this.poolsApi.deletePool(id);
  }

  // Scale Sets
  async listScaleSets(): Promise<ScaleSet[]> {
    const response = await this.scaleSetsApi.listScalesets();
    return response.data || [];
  }

  async getScaleSet(id: number): Promise<ScaleSet> {
    const response = await this.scaleSetsApi.getScaleSet(id.toString());
    return response.data;
  }

  async updateScaleSet(id: number, params: Partial<CreateScaleSetParams>): Promise<ScaleSet> {
    const response = await this.scaleSetsApi.updateScaleSet(id.toString(), params);
    return response.data;
  }

  async deleteScaleSet(id: number): Promise<void> {
    await this.scaleSetsApi.deleteScaleSet(id.toString());
  }

  // Instances
  async listInstances(): Promise<Instance[]> {
    const response = await this.instancesApi.listInstances();
    return response.data || [];
  }

  async getInstance(name: string): Promise<Instance> {
    const response = await this.instancesApi.getInstance(name);
    return response.data;
  }

  async deleteInstance(name: string): Promise<void> {
    await this.instancesApi.deleteInstance(name);
  }

  // Providers
  async listProviders(): Promise<Provider[]> {
    const response = await this.providersApi.listProviders();
    return response.data || [];
  }

  // Compatibility aliases
  async listCredentials(): Promise<ForgeCredentials[]> {
    return this.listAllCredentials();
  }

  async listEndpoints(): Promise<ForgeEndpoint[]> {
    return this.listAllEndpoints();
  }

  // First-run initialization
  async firstRun(params: NewUserParams): Promise<User> {
    const response = await this.firstRunApi.firstRun(params);
    return response.data;
  }

  // Controller management
  async updateController(params: UpdateControllerParams): Promise<ControllerInfo> {
    const response = await this.controllerApi.updateController(params);
    return response.data;
  }
}

// Create a singleton instance
export const generatedGarmApi = new GeneratedGarmApiClient();