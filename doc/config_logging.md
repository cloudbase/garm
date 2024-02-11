# The logging section

GARM has switched to the `slog` package for logging, adding structured logging. As such, we added a dedicated `logging` section to the config to tweak the logging settings. We moved the `enable_log_streamer` and the `log_file` options from the `default` section to the `logging` section. They are still available in the `default` section for backwards compatibility, but they are deprecated and will be removed in a future release.

An example of the new `logging` section:

```toml
[logging]
# Uncomment this line if you'd like to log to a file instead of standard output.
# log_file = "/tmp/runner-manager.log"

# enable_log_streamer enables streaming the logs over websockets
enable_log_streamer = true
# log_format is the output format of the logs. GARM uses structured logging and can
# output as "text" or "json"
log_format = "text"
# log_level is the logging level GARM will output. Available log levels are:
#  * debug
#  * info
#  * warn
#  * error
log_level = "debug"
# log_source will output information about the function that generated the log line.
log_source = false
```

By default GARM logs everything to standard output. You can optionally log to file by adding the `log_file` option to the `logging` section. The `enable_log_streamer` option allows you to stream GARM logs directly to your terminal. Set this option to `true`, then you can use the following command to stream logs:

```bash
garm-cli debug-log
```

The `log_format`, `log_level` and `log_source` options allow you to tweak the logging output. The `log_format` option can be set to `text` or `json`. The `log_level` option can be set to `debug`, `info`, `warn` or `error`. The `log_source` option will output information about the function that generated the log line. All these options influence how the structured logging is output.

This will allow you to ingest GARM logs in a central location such as an ELK stack or similar.