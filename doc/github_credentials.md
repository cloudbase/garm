# Configuring github credentials

The ```github``` config section holds credentials and API endpoint information for accessing the GitHub APIs. Credentials are tied to the instance of GitHub you're using. Whether you're using [github.com](https://github.com) or your own deployment of GitHub Enterprise server, this section is how ```garm``` knows where it should create the runners.

Tying the API endpoint info to the credentials allows us to use the same ```garm``` installation with both [github.com](https://github.com) and private deployments. All you have to do is to add the needed endpoint info (see bellow).

GARM has the option to use both [Personal Access Tokens (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) or a [GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app).


If you'll use a PAT, you'll have to grant access for the following scopes:

* ```public_repo``` - for access to a repository
* ```repo``` - for access to a private repository
* ```admin:org``` - if you plan on using this with an organization to which you have access
* ```manage_runners:enterprise``` - if you plan to use garm at the enterprise level
* ```admin:repo_hook``` - if you want to allow GARM to install webhooks on repositories (optional)
* ```admin:org_hook``` - if you want to allow GARM to install webhooks on organizations (optional)

If you plan to use github apps, you'll need to select the following permissions:

* **Repository permissions**:
  * ```Administration: Read & write```
  * ```Metadata: Read-only```
  * ```Webhooks: Read & write```
* **Organization permissions**:
  * ```Self-hosted runners: Read & write```
  * ```Webhooks: Read & write```

**Note** :warning:: Github Apps are not available at the enterprise level.

The resulting credentials (app or PAT) must be configured in the ```[[github]]``` section of the config. Sample as follows:

```toml
# This is a list of credentials that you can define as part of the repository
# or organization definitions. They are not saved inside the database, as there
# is no Vault integration (yet). This will change in the future.
# Credentials defined here can be listed using the API. Obviously, only the name
# and descriptions are returned.
[[github]]
  name = "gabriel"
  description = "github token or user gabriel"
  # This is the type of authentication to use. It can be "pat" or "app"
  auth_type = "pat"
  [github.pat]
     # This is a personal token with access to the repositories and organizations
    # you plan on adding to garm. The "workflow" option needs to be selected in order
    # to work with repositories, and the admin:org needs to be set if you plan on
    # adding an organization.
    oauth2_token = "super secret token"
  [github.app]
    # This is the app_id of the GitHub App that you want to use to authenticate
    # with the GitHub API.
    # This needs to be changed
    app_id = 1
    # This is the private key path of the GitHub App that you want to use to authenticate
    # with the GitHub API.
    # This needs to be changed
    private_key_path = "/etc/garm/yourAppName.2024-03-01.private-key.pem"
    # This is the installation_id of the GitHub App that you want to use to authenticate
    # with the GitHub API.
    # This needs to be changed
    installation_id = 99
  # base_url (optional) is the URL at which your GitHub Enterprise Server can be accessed.
  # If these credentials are for github.com, leave this setting blank
  base_url = "https://ghe.example.com"
  # api_base_url (optional) is the base URL where the GitHub Enterprise Server API can be accessed.
  # Leave this blank if these credentials are for github.com.
  api_base_url = "https://ghe.example.com"
  # upload_base_url (optional) is the base URL where the GitHub Enterprise Server upload API can be accessed.
  # Leave this blank if these credentials are for github.com, or if you don't have a separate URL
  # for the upload API.
  upload_base_url = "https://api.ghe.example.com"
  # ca_cert_bundle (optional) is the CA certificate bundle in PEM format that will be used by the github
  # client to talk to the API. This bundle will also be sent to all runners as bootstrap params.
  # Use this option if you're using a self signed certificate.
  # Leave this blank if you're using github.com or if your certificate is signed by a valid CA.
  ca_cert_bundle = "/etc/garm/ghe.crt"
```

The double parenthesis means that this is an array. You can specify the ```[[github]]``` section multiple times, with different tokens from different users, or with different access levels. You will then be able to list the available credentials using the API, and reference these credentials when adding repositories or organizations.

The API will only ever return the name and description to the API consumer.
