# OIDC Authentication

GARM supports OpenID Connect (OIDC) authentication, allowing users to authenticate using external identity providers such as Google, Okta, Azure AD, Keycloak, and other OIDC-compliant providers.

## Configuration

To enable OIDC authentication, add the `[oidc]` section to your GARM configuration file:

```toml
[oidc]
# Enable OIDC authentication
enable = true

# The OIDC provider's issuer URL
# Examples:
#   - Google: https://accounts.google.com
#   - Okta: https://your-domain.okta.com
#   - Azure AD: https://login.microsoftonline.com/{tenant}/v2.0
#   - Keycloak: https://your-keycloak-server/realms/{realm}
issuer_url = "https://accounts.google.com"

# OAuth2 client ID from your identity provider
client_id = "your-client-id"

# OAuth2 client secret from your identity provider
client_secret = "your-client-secret"

# The callback URL where the identity provider will redirect after authentication
# This must match the redirect URI configured in your identity provider
redirect_url = "https://your-garm-server/api/v1/auth/oidc/callback"

# OAuth2 scopes to request (optional)
# Defaults to ["openid", "email", "profile"] if not specified
scopes = ["openid", "email", "profile"]

# Restrict login to users with email addresses from specific domains (optional)
# If empty, all authenticated users are allowed
allowed_domains = ["example.com", "yourcompany.com"]

# Enable Just-In-Time (JIT) user creation on first OIDC login (optional)
# If true, new users will be automatically created when they first authenticate via OIDC
# If false, users must be pre-created in GARM before they can log in
jit_user_creation = true

# Set whether JIT-created users should be admins (optional)
# Only applies when jit_user_creation is true
default_user_admin = false
```

## API Endpoints

OIDC authentication adds the following API endpoints:

### Login Endpoint

```
GET /api/v1/auth/oidc/login
```

Initiates the OIDC login flow by redirecting the user to the identity provider's authorization endpoint.

**Response:** HTTP 302 redirect to the identity provider

### Callback Endpoint

```
GET /api/v1/auth/oidc/callback
```

Handles the callback from the identity provider after successful authentication.

**Query Parameters:**
- `code` - Authorization code from the identity provider
- `state` - State parameter for CSRF protection

**Success Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Error Responses:**
- `400 Bad Request` - Missing parameters, invalid state, or token exchange failure
- `401 Unauthorized` - User not allowed (domain restriction or user disabled)

## How It Works

1. **User initiates login**: The user navigates to `/api/v1/auth/oidc/login`
2. **Redirect to IdP**: GARM redirects the user to the identity provider with a state parameter
3. **User authenticates**: The user authenticates with their identity provider
4. **Callback**: The IdP redirects back to `/api/v1/auth/oidc/callback` with an authorization code
5. **Token exchange**: GARM exchanges the code for tokens with the IdP
6. **User lookup/creation**: GARM looks up the user by email, or creates one if JIT is enabled
7. **JWT issued**: GARM issues a JWT token for subsequent API requests

## Setting Up Identity Providers

### Google

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create or select a project
3. Navigate to "APIs & Services" > "Credentials"
4. Create an OAuth 2.0 Client ID
5. Add your callback URL: `https://your-garm-server/api/v1/auth/oidc/callback`
6. Copy the Client ID and Client Secret to your GARM config

### Okta

1. Log in to your Okta Admin Console
2. Navigate to "Applications" > "Create App Integration"
3. Select "OIDC - OpenID Connect" and "Web Application"
4. Add your callback URL
5. Copy the Client ID and Client Secret

### Azure AD

1. Go to the [Azure Portal](https://portal.azure.com/)
2. Navigate to "Azure Active Directory" > "App registrations"
3. Create a new registration
4. Add a redirect URI for "Web" platform
5. Create a client secret under "Certificates & secrets"

### Keycloak

1. Log in to your Keycloak Admin Console
2. Select or create a realm
3. Navigate to "Clients" and create a new client
4. Set the Root URL and Valid Redirect URIs
5. Copy the Client ID and Client Secret from the "Credentials" tab

## Security Considerations

- **HTTPS Required**: Always use HTTPS for the redirect URL in production
- **Client Secret**: Keep the client secret secure and never expose it
- **Domain Restrictions**: Use `allowed_domains` to restrict access to specific email domains
- **JIT User Creation**: Consider disabling JIT creation (`jit_user_creation = false`) for tighter access control
- **State Validation**: GARM validates the state parameter to prevent CSRF attacks
- **Token Expiration**: OIDC state tokens expire after 10 minutes

## Troubleshooting

### "OIDC authentication is not enabled"
Ensure `enable = true` in the `[oidc]` section and restart GARM.

### "failed to create OIDC provider"
Check that the `issuer_url` is correct and accessible from the GARM server.

### "email domain not allowed"
The user's email domain is not in the `allowed_domains` list.

### "user not found and JIT creation disabled"
Enable `jit_user_creation = true` or pre-create the user in GARM.

