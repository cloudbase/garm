# Writing an external provider

External provider enables you to write a fully functional provider, using any scripring or programming language. Garm will then call your executable to manage the lifecycle of the instances hosting the runners. This document describes the API that an executable needs to implement to be usable by ```garm```.

## Environment variables

When ```garm``` calls your executable, a number of environment variables are set, depending on the operation. There are two environment variable will always be set regardless of operation. Those variables are:

  * ```GARM_COMMAND```
  * ```GARM_PROVIDER_CONFIG_FILE```

The following are variables that are specific to some operations:

  * ```GARM_CONTROLLER_ID```
  * ```GARM_POOL_ID```
  * ```GARM_INSTANCE_ID```

### The GARM_COMMAND variable

The ```GARM_COMMAND``` environment variable will be set to one of the operations defined in the interface. When your executable is called, you'll need to look at this variable to know which operation you need to execute.

### The GARM_PROVIDER_CONFIG_FILE variable

The ```GARM_PROVIDER_CONFIG_FILE``` variable will contain a path on disk to a file that can contain whatever configuration your executable needs. For example, in the case of the OpenStack external provider, this file contains variables that you would normally find in a ```keystonerc``` file, used to access an OpenStack cloud. But you can use it to add any extra configuration you need.

In your executable, you could implement something like this:

```bash
if [ -f "${GARM_PROVIDER_CONFIG_FILE}" ];then
    source "${GARM_PROVIDER_CONFIG_FILE}"
fi
```

Which would make the contents of that config available to you. Then you could implement the needed operations:

```bash
case "${GARM_COMMAND}" in
    "CreateInstance")
        # Run the create instance code
        ;;
    "DeleteInstance")
        # Run the delete instance code
        ;;
    # .... the rest of the operations detailed in next sections ....
    *)
        # handle unknown command
        echo "unknown command ${GARM_COMMAND}"
        exit 1
        ;;
esac
```

### The GARM_CONTROLLER_ID variable

The ```GARM_CONTROLLER_ID``` variable is set for two operations:

  * CreateInstance
  * RemoveAllInstances

This variable contains the ```UUID4``` identifying a ```garm``` installation. Whenever you start up ```garm``` for the first time, a new ```UUID4``` is generated and saved in ```garm's``` database. This ID is meant to be used to track all resources created by ```garm``` within a provider. That way, if you decide to tare it all down, you have a way of identifying what was created by one particular installation of ```garm```.

This is useful if various teams from your company use the same credentials to access a cloud. You won't accidentally clobber someone else's resources.

In most clouds you can attach ```tags``` to resources. You can use the controller ID as one of the tagg suring the ```CreateInstance``` operation.

# The GARM_POOL_ID variable

The ```GARM_POOL_ID``` anvironment variable is an ```UUID4``` describing the pool in which a runner is created. This variable is set in two operations:

  * CreateInstance
  * ListInstances

As is with ```GARM_CONTROLLER_ID```, this ID can also be attached as a tag in most clouds.

### The GARM_INSTANCE_ID variable

The ```GARM_INSTANCE_ID``` environment variable is used in four operations:

  * GetInstance
  * DeleteInstance
  * Start
  * Stop

It contains the ```provider_id``` of the instance. The ```provider_id``` is a unique identifier, specific to the IaaS in which the compute resource was created. In OpenStack, it's an ```UUID4```, while in LXD, it's the virtual machine's name.

We need this ID whenever we need to execute an operation that targets one specific runner. 

# Operations

The operations that a provider must implement are described in the ```Provider``` interface available [here](https://github.com/cloudbase/garm/blob/main/runner/common/provider.go#L22-L39). The external provider implements this interface, and delegates each operation to your external executable. These operations are:

  * CreateInstance
  * DeleteInstance
  * GetInstance
  * ListInstances
  * RemoveAllInstances
  * Stop
  * Start

The ```AsParams()``` function does not need to be implemented by the external executable.

## CreateInstance

The ```CreateInstance``` command has the most moving parts. The ideal external provider is one that will create all required resources for a fully functional instance, will start the instance. Waiting for the instance to start is not necessary. If the instance can reach the ```callback_url``` configured in ```garm```, it will update it's own status when it boots.

But aside from creating resources, the ideal external provider is also idempotent, and will clean up after itself in case of failure. If for any reason the executable will fail to create the instance, any dependency that it has created up to the point of failure, should be cleaned up before returning an error code.

At the very least, it must be able to clean up those resources, if it is called with the ```DeleteInstance``` command by ```garm```. Garm will retry creating a failed instance. Before it tries again, it will attempt to run a ```DeleteInstance``` using the ```provider_id``` returned by your executable.

If your executable failed before a ```provider_id``` could be supplied, ```garm``` will send the name of the instance as a ```GARM_INSTANCE_ID``` environment variable.

Your external provider will need to be able to handle both. The instance name generated by ```garm``` will be unique (contains a UUID4), so it's fairly safe to use when deleting instances.

### CreateInstance inputs

The ```CreateInstance``` command is the only command that receives information using, environment variables and standard input. The available environment variables are:

  * GARM_PROVIDER_CONFIG_FILE - Config file specific to your executable
  * GARM_COMMAND - the command we need to run
  * GARM_CONTROLLER_ID - The unique ID of the ```garm``` installation
  * GARM_POOL_ID - The unique ID of the pool this node is a part of

The information sent in via standard input is a ```json``` serialized instance of the [BootstrapInstance structure](https://github.com/cloudbase/garm/blob/main/params/params.go#L80-L103)

Here is a sample of that:

```json
{
  "name": "garm-fc7b3174-9695-460e-b9c7-ae75ee217b53",
  "tools": [
    {
      "os": "osx",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-osx-x64-2.291.1.tar.gz",
      "filename": "actions-runner-osx-x64-2.291.1.tar.gz",
      "sha256_checksum": "1ed51d6f35af946e97bb1e10f1272197ded20dd55186ae463563cd2f58f476dc"
    },
    {
      "os": "linux",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-linux-x64-2.291.1.tar.gz",
      "filename": "actions-runner-linux-x64-2.291.1.tar.gz",
      "sha256_checksum": "1bde3f2baf514adda5f8cf2ce531edd2f6be52ed84b9b6733bf43006d36dcd4c"
    },
    {
      "os": "win",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-win-x64-2.291.1.zip",
      "filename": "actions-runner-win-x64-2.291.1.zip",
      "sha256_checksum": "2a504f852b0ab0362d08a36a84984753c2ac159ef17e5d1cd93f661ecd367cbd"
    },
    {
      "os": "linux",
      "architecture": "arm",
      "download_url": "https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-linux-arm-2.291.1.tar.gz",
      "filename": "actions-runner-linux-arm-2.291.1.tar.gz",
      "sha256_checksum": "a78e86ba6428a28733730bdff3a807480f9eeb843f4c64bd1bbc45de13e61348"
    },
    {
      "os": "linux",
      "architecture": "arm64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-linux-arm64-2.291.1.tar.gz",
      "filename": "actions-runner-linux-arm64-2.291.1.tar.gz",
      "sha256_checksum": "c4823bd8322f80cb24a311ef49273f0677ff938530248242de7df33800a22900"
    }
  ],
  "repo_url": "https://github.com/gabriel-samfira/scripts",
  "github_runner_access_token": "super secret token",
  "callback-url": "https://garm.example.com/api/v1/callbacks/status",
  "instance-token": "super secret JWT token",
  "ssh-keys": null,
  "arch": "amd64",
  "flavor": "m1.small",
  "image": "050f1e00-7eab-4f47-b10b-796df34d2e6b",
  "labels": [
    "ubuntu",
    "simple-runner",
    "repo-runner",
    "self-hosted",
    "x64",
    "linux",
    "runner-controller-id:f9286791-1589-4f39-a106-5b68c2a18af4",
    "runner-pool-id:fb25f308-7ad2-4769-988e-6ec2935f642a"
  ],
  "pool_id": "fb25f308-7ad2-4769-988e-6ec2935f642a"
}
```

In your executable you can read in this blob, by using something like this:

```bash
# Test if the stdin file descriptor is opened
if [ ! -t 0 ]
then
    # Read in the information from standard in
    INPUT=$(cat -)
fi
```

Then you can easily parse it. If you're using ```bash```, you can use the amazing [jq json processor](https://stedolan.github.io/jq/). Other programming languages have suitable libraries that can handle ```json```.

You will have to parse the bootstrap params, verify that the requested image exists, gather operating system information, CPU architecture information and using that information, you will need to select the appropriate tools for the arch/OS combination you are deploying.

Refer to the OpenStack or Azure providers available in the [providers.d](../contrib/providers.d/) folder.

### CreateInstance outputs

On success, your executable is expected to print to standard output a json that can be unserialized into an ```Instance{}``` structure [defined here](https://github.com/cloudbase/garm/blob/main/params/params.go#L43-L78).

Not all fields are expected to be populated by the provider. The ones that should be set are:

```json
{
  "provider_id": "88818ff3-1fca-4cb5-9b37-84bfc3511ea6",
  "name": "garm-0542a982-4a0d-4aca-aef0-d736c96f61ca",
  "os_type": "linux",
  "os_name": "ubuntu",
  "os_version": "20.04",
  "os_arch": "x86_64",
  "status": "running",
  "pool_id": "41c4a43a-acee-493a-965b-cf340b2c775d",
  "provider_fault": ""
}
```

In case of error, ```garm``` expects at the very least to see a non-zero exit code. If possible, your executable should return as much information as possible via the above ```json```, with the ```status``` field set to ```error``` and the ```provider_fault``` set to a meaningful error message describing what has happened. That information will be visible when doing a:

```bash
garm-cli runner show <runner name>
```

## DeleteInstance

The ```DeleteInstance``` command will permanently remove an instance from the cloud provider.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_INSTANCE_ID

This command is not expected to output anything. On success it should simply ```exit 0```.

If the target instance does not exist in the provider, this command is expected to be a noop.

## GetInstance

NOTE: This operation is currently not use by ```garm```, but should be implemented.

The ```GetInstance``` command will return details about the instance, as seen by the provider.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_INSTANCE_ID

On success, this command is expected to return a valid ```json``` that can be unserialized into an ```Instance{}``` structure (see CreateInstance). If possible, IP addresses allocated to the VM should be returned in adition to the sample ```json``` printed above.

On failure, this command is expected to return a non-zero exit code.

## ListInstances

NOTE: This operation is currently not use by ```garm```, but should be implemented.

The ```ListInstances``` command will print to standard output, a json that is unserializable into an **array** of ```Instance{}```.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_POOL_ID

This command must list all instances that have been tagged with the value in ```GARM_POOL_ID```.

On success, a ```json``` is expected on standard output.

On failure, a non-zero exit code is expected.

## RemoveAllInstances

NOTE: This operation is currently not use by ```garm```, but should be implemented.

The ```RemoveAllInstances``` operation will remove all resources created in a cloud that have been tagged with the ```GARM_CONTROLLER_ID```.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_CONTROLLER_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.

## Start

The ```Start``` operation will start the virtual machine in the selected cloud.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_INSTANCE_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.


## Stop

NOTE: This operation is currently not use by ```garm```, but should be implemented.

The ```Stop``` operation will stop the virtual machine in the selected cloud.

Available environment variables:

  * GARM_COMMAND
  * GARM_PROVIDER_CONFIG_FILE
  * GARM_INSTANCE_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.
