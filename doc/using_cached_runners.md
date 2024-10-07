# Using Cached Runners

## GitHub Action Runners and GARM

When a new instance is created by garm, it usually downloads the latest available GitHub action runner binary, installs the requirements and starts it afterwards. This can be a time consuming task that quickly adds up when a lot of instances are created by garm throughout the day. Therefore it is recommended to include the GitHub action runner binary inside of the used image.

GARM supports cached runners on Linux and Windows images, in a simple manner. GARM verifies if the runner path exists (`C:\actions-runner` or `/home/runner/actions-runner`) on the chosen image, thus knowing if it needs to create the path and download the runner or use the existent runner. In order to simplify setup and validation of the runner, the check is based on the user properly creating, downloading and installing the runner in the predefined path on the target OS.

>**NOTE:** More about these paths will be presented below in the sections for each target OS.

### Cached Runners on Linux Images

On a Linux image, the cached runner is expected by GARM to be setup in a static predefined way. It expects the cached runner to be installed in the `/home/runner/actions-runner` directory. Thus, the user needs to configure its custom image properly in order for GARM to use the cached runner and not download the latest available GitHub action runner binary.

In order to configure a cached GitHub actions runner to work with GARM, the following steps need to be followed:

1. The `actions-runner`directory needs to be created inside the `/home/runner` directory (home path for the garm runner)
2. Download the wanted version of the runner package
3. Extract the installer inside the `actions-runner` directory

> **NOTE:** These are based on the steps described on the [actions/runner](https://github.com/actions/runner/releases) repository about installing the GitHub action runner on the Linux x64. The full list of commands looks like this:

```bash
# Create a folder
mkdir actions-runner && cd actions-runner
# Download the latest runner package
curl -O -L https://github.com/actions/runner/releases/download/v2.320.0/actions-runner-linux-x64-2.320.0.tar.gz
# Extract the installer
tar xzf ./actions-runner-linux-x64-2.320.0.tar.gz
```

### Cached Runners on Windows Images

On a Windows image, the cached runner is expected by GARM to be setup in a static predefined way. It expects the cached runner to be installed in the `C:\actions-runner\` folder. Thus, the user needs to configure its custom image properly in order for GARM to use the cached runner and not download the latest available GitHub action runner binary.

In order to configure a cached GitHub actions runner to work with GARM, the following steps need to be followed:

1. Create the folder `actions-runner` inside the root folder (`C:\`).
2. Download the wanted version of runner package
3. Extract the installer in the folder created at step 1 (`C:\actions-runner\`)

> **NOTE:** These are based on the steps described on the [actions/runner](https://github.com/actions/runner/releases) repository about installing the GitHub action runner on the Windows x64. The full list of commands looks like this:

```powershell
# Create a folder under the drive root
mkdir \actions-runner ; cd \actions-runner
# Download the latest runner package
Invoke-WebRequest -Uri https://github.com/actions/runner/releases/download/v2.320.0/actions-runner-win-x64-2.320.0.zip -OutFile actions-runner-win-x64-2.320.0.zip
# Extract the installer
Add-Type -AssemblyName System.IO.Compression.FileSystem ;
[System.IO.Compression.ZipFile]::ExtractToDirectory("$PWD\actions-runner-win-x64-2.320.0.zip", "$PWD")
```