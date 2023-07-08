# The JWT authentication config section

This section configures the JWT authentication used by the API server. GARM is currently a single user system and that user has the right to do anything and everything GARM is capable of. As a result, the JWT auth we have does not include a refresh token. The token is valid for the duration of the time to live (TTL) set in the config file. Once the token expires, you will need to log in again.

It is recommended that the secret be a long, randomly generated string. Changing the secret at any time will invalidate all existing tokens.

```toml
[jwt_auth]
# A JWT token secret used to sign tokens.
# Obviously, this needs to be changed :).
secret = ")9gk_4A6KrXz9D2u`0@MPea*sd6W`%@5MAWpWWJ3P3EqW~qB!!(Vd$FhNc*eU4vG"

# Time to live for tokens. Both the instances and you will use JWT tokens to
# authenticate against the API. However, this TTL is applied only to tokens you
# get when logging into the API. The tokens issued to the instances we manage,
# have a TTL based on the runner bootstrap timeout set on each pool. The minimum
# TTL for this token is 24h.
time_to_live = "8760h"
```