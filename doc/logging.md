# Logging

By default, GARM is logging only on standard output.

If you would like GARM to use a logging file instead, you can use the `log_file` configuration option:

```toml
[default]
# Use this if you'd like to log to a file instead of standard output.
log_file = "/tmp/runner-manager.log"
```

## Rotating log files

If GARM uses a log file, by default it will rotate it when it reaches 500MB or 28 days, whichever comes first.

However, if you want to manually rotate the log file, you can send a `SIGHUP` signal to the GARM process.
