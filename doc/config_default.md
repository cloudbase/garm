# The default config section

The `default` config section holds configuration options that don't need a category of their own, but are essential to the operation of the service. In this section we will detail each of the options available in the `default` section.

```toml
[default]
# This URL is used by instances to send back status messages as they install
# the github actions runner. Status messages can be seen by querying the
# runner status in garm.
# Note: If you're using a reverse proxy in front of your garm installation,
# this URL needs to point to the address of the reverse proxy. Using TLS is
# highly encouraged.
callback_url = "https://garm.example.com/api/v1/callbacks"

# This URL is used by instances to retrieve information they need to set themselves
# up. Access to this URL is granted using the same JWT token used to send back
# status updates. Once the instance transitions to "installed" or "failed" state,
# access to both the status and metadata endpoints is disabled.
# Note: If you're using a reverse proxy in front of your garm installation,
# this URL needs to point to the address of the reverse proxy. Using TLS is
# highly encouraged.
metadata_url = "https://garm.example.com/api/v1/metadata"

# Uncomment this line if you'd like to log to a file instead of standard output.
# log_file = "/tmp/runner-manager.log"

# Enable streaming logs via web sockets. Use garm-cli debug-log.
enable_log_streamer = false

# Enable the golang debug server. See the documentation in the "doc" folder for more information.
debug_server = false
```

## The callback_url option

Your runners will call back home with status updates as they install. Once they are set up, they will also send the GitHub agent ID they were allocated. You will need to configure the ```callback_url``` option in the ```garm``` server config. This URL needs to point to the following API endpoint:

  ```txt
  POST /api/v1/callbacks/status
  ```

Example of a runner sending status updates:

  ```bash
  garm-cli runner show garm-DvxiVAlfHeE7
  +-----------------+------------------------------------------------------------------------------------+
  | FIELD           | VALUE                                                                              |
  +-----------------+------------------------------------------------------------------------------------+
  | ID              | 16b96ba2-d406-45b8-ab66-b70be6237b4e                                               |
  | Provider ID     | garm-DvxiVAlfHeE7                                                                  |
  | Name            | garm-DvxiVAlfHeE7                                                                  |
  | OS Type         | linux                                                                              |
  | OS Architecture | amd64                                                                              |
  | OS Name         | ubuntu                                                                             |
  | OS Version      | jammy                                                                              |
  | Status          | running                                                                            |
  | Runner Status   | idle                                                                               |
  | Pool ID         | 8ec34c1f-b053-4a5d-80d6-40afdfb389f9                                               |
  | Addresses       | 10.198.117.120                                                                     |
  | Status Updates  | 2023-07-08T06:26:46: runner registration token was retrieved                       |
  |                 | 2023-07-08T06:26:46: using cached runner found in /opt/cache/actions-runner/latest |
  |                 | 2023-07-08T06:26:50: configuring runner                                            |
  |                 | 2023-07-08T06:26:56: runner successfully configured after 1 attempt(s)             |
  |                 | 2023-07-08T06:26:56: installing runner service                                     |
  |                 | 2023-07-08T06:26:56: starting service                                              |
  |                 | 2023-07-08T06:26:57: runner successfully installed                                 |
  +-----------------+------------------------------------------------------------------------------------+

  ```

This URL must be set and must be accessible by the instance. If you wish to restrict access to it, a reverse proxy can be configured to accept requests only from networks in which the runners ```garm``` manages will be spun up. This URL doesn't need to be globally accessible, it just needs to be accessible by the instances.

For example, in a scenario where you expose the API endpoint directly, this setting could look like the following:

  ```toml
  callback_url = "https://garm.example.com/api/v1/callbacks"
  ```

Authentication is done using a short-lived JWT token, that gets generated for a particular instance that we are spinning up. That JWT token grants access to the instance to only update it's own status and to fetch metadata for itself. No other API endpoints will work with that JWT token. The validity of the token is equal to the pool bootstrap timeout value (default 20 minutes) plus the garm polling interval (5 minutes).

There is a sample ```nginx``` config [in the testdata folder](/testdata/nginx-server.conf). Feel free to customize it whichever way you see fit.

## The metadata_url option

The metadata URL is the base URL for any information an instance may need to fetch in order to finish setting itself up. As this URL may be placed behind a reverse proxy, you'll need to configure it in the ```garm``` config file. Ultimately this URL will need to point to the following ```garm``` API endpoint:

  ```bash
  GET /api/v1/metadata
  ```

This URL needs to be accessible only by the instances ```garm``` sets up. This URL will not be used by anyone else. To configure it in ```garm``` add the following line in the ```[default]``` section of your ```garm``` config:

  ```toml
  metadata_url = "https://garm.example.com/api/v1/metadata"
  ```

## The debug_server option

GARM can optionally enable the golang profiling server. This is useful if you suspect garm may be bottlenecking in any way. To enable the profiling server, add the following section to the garm config:

```toml
[default]

debug_server = true
```

And restart garm. You can then use the following command to start profiling:

```bash
go tool pprof http://127.0.0.1:9997/debug/pprof/profile?seconds=120
```

Important note on profiling when behind a reverse proxy. The above command will hang for a fairly long time. Most reverse proxies will timeout after about 60 seconds. To avoid this, you should only profile on localhost by connecting directly to garm.

It's also advisable to exclude the debug server URLs from your reverse proxy and only make them available locally.

Now that the debug server is enabled, here is a blog post on how to profile golang applications: https://blog.golang.org/profiling-go-programs


## The log_file option

By default, GARM logs everything to standard output.

You can optionally log to file by adding the following to your config file:

```toml
[default]
# Use this if you'd like to log to a file instead of standard output.
log_file = "/tmp/runner-manager.log"
```

### Rotating log files

GARM automatically rotates the log if it reaches 500 MB in size or 28 days, whichever comes first.

However, if you want to manually rotate the log file, you can send a `SIGHUP` signal to the GARM process.

You can add the following to your systemd unit file to enable `reload`:

```ini
[Service]
ExecReload=/bin/kill -HUP $MAINPID
```

Then you can simply:

```bash
systemctl reload garm
```

## The enable_log_streamer option

This option allows you to stream garm logs directly to your terminal. Set this option to true, then you can use the following command to stream logs:

```bash
garm-cli debug-log
```

An important note on enabling this option when behind a reverse proxy. The log streamer uses websockets to stream logs to you. You will need to configure your reverse proxy to allow websocket connections. If you're using nginx, you will need to add the following to your nginx `server` config:

```nginx
location /api/v1/ws {
    proxy_pass http://garm_backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
}
```