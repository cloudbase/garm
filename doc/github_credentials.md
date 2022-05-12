# Configuring github credentials

Garm needs a [Personal Access Token (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) to create runner registration tokens, list current self hosted runners and potentially remove them if they become orphaned (the VM was manually removed on the provider).

From the list of scopes, you will need to select:

  * ```public_repo``` - for access to a repository
  * ```repo``` - for access to a private repository
  * ```admin:org``` - if you plan on using this with an organization to which you have access

The resulting token must be configured in the ```[[github]]``` section of the config. Sample as follows:

```toml
# This is a list of credentials that you can define as part of the repository
# or organization definitions. They are not saved inside the database, as there
# is no Vault integration (yet). This will change in the future.
# Credentials defined here can be listed using the API. Obviously, only the name
# and descriptions are returned.
[[github]]
  name = "gabriel"
  description = "github token or user gabriel"
  # This is a personal token with access to the repositories and organizations
  # you plan on adding to garm. The "workflow" option needs to be selected in order
  # to work with repositories, and the admin:org needs to be set if you plan on
  # adding an organization.
  oauth2_token = "super secret token"
```

The double paranthesis means that this is an array. You can specify the ```[[github]]``` section multiple times, with different tokens from different users, or with different access levels. You will then be able to list the available credentials using the API, and reference these credentials when adding repositories or organizations.

The API will only ever return the name and description to the API consumer.
