# Performance Optimization

Runner startup time is the main performance concern in GARM. Each new runner must boot an instance, download the GitHub Actions runner binary, install dependencies, and register. These tips help minimize that time.

## Cache the runner binary in images

The biggest time saver. Pre-install the GitHub Actions runner binary in your image so GARM doesn't download it on every boot.

GARM checks for cached runners at these paths:

- **Linux:** `/home/runner/actions-runner`
- **Windows:** `C:\actions-runner\`

If the path exists, GARM uses the cached runner instead of downloading.

### Linux: LXD example

```bash
# Launch a temporary container from your base image
lxc launch ubuntu:22.04 temp

# Install the runner inside the container
lxc exec temp -- bash -c '
  mkdir -p /home/runner/actions-runner
  cd /home/runner/actions-runner
  curl -O -L https://github.com/actions/runner/releases/download/v2.320.0/actions-runner-linux-x64-2.320.0.tar.gz
  tar xzf ./actions-runner-linux-x64-2.320.0.tar.gz
'

# Stop, publish as image, clean up
lxc stop temp
lxc publish temp --alias ubuntu-22.04-runner-2.320.0
lxc delete temp

# Update your pool to use the new image
garm-cli pool update <POOL_ID> --image=ubuntu-22.04-runner-2.320.0
```

### Windows

```powershell
mkdir C:\actions-runner
cd C:\actions-runner
Invoke-WebRequest -Uri https://github.com/actions/runner/releases/download/v2.320.0/actions-runner-win-x64-2.320.0.zip -OutFile actions-runner.zip
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory("$PWD\actions-runner.zip", "$PWD")
```

## Disable OS updates during bootstrap

By default, GARM configures cloud-init to update packages on first boot. Disable this to save time:

```bash
garm-cli pool update <POOL_ID> --extra-specs='{"disable_updates": true}'
```

## LXD/Incus-specific optimizations

### Storage driver

Choose a storage driver that supports **optimized image storage** and **optimized instance creation**. Check your current driver:

```bash
lxc storage list
```

See the [LXD storage driver documentation](https://linuxcontainers.org/lxd/docs/latest/reference/storage_drivers/) for feature comparisons.

### Enable shiftfs

Unprivileged LXD containers require filesystem remapping, which is slow for large images. Enable shiftfs to skip this:

```bash
snap set lxd shiftfs.enable=true
systemctl reload snap.lxd.daemon
```

This can reduce startup time from minutes to seconds for large images. The preferred alternative is `idmapped mounts` (kernel 5.12+), but not all filesystems support it yet, so shiftfs remains the more broadly compatible option.

> [!IMPORTANT]
> When shiftfs is enabled, mounting volumes between host and container may require extra steps to maintain security. See the [LXD shiftfs discussion](https://discuss.linuxcontainers.org/t/trying-out-shiftfs/5155) for details.
