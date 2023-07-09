# The API server config section

This section allows you to configure the GARM API server. The API server is responsible for serving all the API endpoints used by the `garm-cli`, the runners that phone home their status and by GitHub when it sends us webhooks.

The config options are fairly straight forward.

```toml
[apiserver]
  # Bind the API to this IP
  bind = "0.0.0.0"
  # Bind the API to this port
  port = 9997
  # Whether or not to set up TLS for the API endpoint. If this is set to true,
  # you must have a valid apiserver.tls section.
  use_tls = false
  # Set a list of allowed origins
  # By default, if this option is ommited or empty, we will check
  # only that the origin is the same as the originating server.
  # A literal of "*" will allow any origin
  cors_origins = ["*"]
  [apiserver.tls]
    # Path on disk to a x509 certificate bundle.
    # NOTE: if your certificate is signed by an intermediary CA, this file
    # must contain the entire certificate bundle needed for clients to validate
    # the certificate. This usually means concatenating the certificate and the
    # CA bundle you received.
    certificate = ""
    # The path on disk to the corresponding private key for the certificate.
    key = ""
```

The GARM API server has the option to enable TLS, but I suggest you use a reverse proxy and enable TLS termination in that reverse proxy. There is an `nginx` sample in this repository with TLS termination enabled.

You can of course enable TLS in both garm and the reverse proxy. The choice is yours.