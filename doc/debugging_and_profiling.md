# Debugging and profiling

GARM can optionally enable the golang profiling server. You can then use the usual `go tool pprof` command to start profiling. This is useful if you suspect garm may be bottlenecking in any way. To enable the profiling server, add the following section to the garm config:

```toml
[default]

debug_server = true
```

Then restarg garm. You can then use the following command to start profiling:

```bash
go tool pprof http://127.0.0.1:9997/debug/pprof/profile?seconds=120
```

Important note on profiling when behind a reverse proxy. The above command will hang for a fairly long time. Most reverse proxies will timeout after about 60 seconds. To avoid this, you should only profile on localhost by connecting directly to garm.

It's also advisable to exclude the debug server URLs from your reverse proxy and only make them available locally.

Now that the debug server is enabled, here is a blog post on how to profile golang applications: https://blog.golang.org/profiling-go-programs

