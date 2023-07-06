# Logging

By default, GARM logs everything to standard output.

You can optionally log to file by adding the following to your config file:

```toml
[default]
# Use this if you'd like to log to a file instead of standard output.
log_file = "/tmp/runner-manager.log"
```

## Rotating log files

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