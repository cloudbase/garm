# Running macOS runners

GARM can drive self-hosted macOS runners on Apple Silicon through an external
provider that manages Tart virtual machines (for example
[garm-provider-tart](https://github.com/yanksyoon/garm-provider-tart)). macOS
guests differ from Linux and Windows guests in a few ways that matter when you
create a pool.

## No cloud-init

macOS guests have no cloud-init, so the provider cannot rely on the standard
cloud-config path. Instead, a macOS pool must set `--runner-install-template` to
a runner-install template that works on macOS. GARM renders a POSIX bash wrapper
into `extra_specs["runner_install_template"]` and the provider runs it inside
the guest. The same wrapper is used for both Linux and Darwin guests: it is
plain POSIX bash and there is no darwin-specific variant.

## Licensing

Apple's macOS software licence agreement allows at most two additional macOS
guests per Apple-branded host for the permitted uses. This is a licence
condition, not something GARM enforces: keep a host's pool size within that
limit yourself.

## Host prerequisites

Since macOS 15, the login keychain must be unlocked for a Tart VM to start.
Configure auto-login or a boot LaunchDaemon that runs `security unlock-keychain`
on each host.
