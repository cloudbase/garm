# Writing an external provider

External provider enables you to write a fully functional provider, using any scripting or programming language. Garm will call your executable to manage the lifecycle of the instances hosting the runners. This document describes the API that an executable needs to implement to be usable by ```garm```.

## Environment variables

When ```garm``` calls your executable, a number of environment variables are set, depending on the operation. There are three environment variables that will always be set regardless of operation. Those variables are:

* ```GARM_COMMAND```
* ```GARM_PROVIDER_CONFIG_FILE```
* ```GARM_CONTROLLER_ID```

The following are variables that are specific to some operations:

* ```GARM_POOL_ID```
* ```GARM_INSTANCE_ID```

### The GARM_COMMAND variable

The ```GARM_COMMAND``` environment variable will be set to one of the operations defined in the interface. When your executable is called, you'll need to inspect this variable to know which operation you need to execute.

### The GARM_PROVIDER_CONFIG_FILE variable

The ```GARM_PROVIDER_CONFIG_FILE``` variable will contain a path on disk to a file that can contain whatever configuration your executable needs. For example, in the case of the [sample OpenStack external provider](../contrib/providers.d/openstack/keystonerc), this file contains variables that you would normally find in a ```keystonerc``` file, used to access an OpenStack cloud. But you can use it to add any extra configuration you need.

The config is opaque to ```garm``` itself. It only has meaning for your external provider.

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

The ```GARM_CONTROLLER_ID``` variable is set for all operations.

When garm first starts up, it generates a unique ID that identifies it as an instance. This ID is passed to the provider and should always be used to tag resources in whichever cloud you write your provider for. This ensures that if you have multiple garm installations, one particular deployment of garm will never touch any resources it did not create.

In most clouds you can attach ```tags``` to resources. You can use the controller ID as one of the tags during the ```CreateInstance``` operation.

### The GARM_POOL_ID variable

The ```GARM_POOL_ID``` environment variable is a ```UUID4``` describing the pool in which a runner is created. This variable is set in two operations:

* CreateInstance
* ListInstances

As with the ```GARM_CONTROLLER_ID```, this ID **must** also be attached as a tag or whichever mechanism your target cloud supports, to identify the pool to which the resources (in most cases the VMs) belong to.

### The GARM_INSTANCE_ID variable

The ```GARM_INSTANCE_ID``` environment variable is used in four operations:

* GetInstance
* DeleteInstance
* Start
* Stop

It contains the ```provider_id``` of the instance. The ```provider_id``` is a unique identifier, specific to the IaaS in which the compute resource was created. In OpenStack, it's an ```UUID4```, while in LXD, it's the virtual machine's name.

We need this ID whenever we need to execute an operation that targets one specific runner.

## Operations

The operations that a provider must implement are described in the ```Provider``` [interface available here](https://github.com/cloudbase/garm/blob/223477c4ddfb6b6f9079c444d2f301ef587f048b/runner/providers/external/execution/interface.go#L9-L27). The external provider implements this interface, and delegates each operation to your external executable. [These operations are](https://github.com/cloudbase/garm/blob/223477c4ddfb6b6f9079c444d2f301ef587f048b/runner/providers/external/execution/commands.go#L5-L13):

* CreateInstance
* DeleteInstance
* GetInstance
* ListInstances
* RemoveAllInstances
* Stop
* Start

## CreateInstance

The ```CreateInstance``` command has the most moving parts. The ideal external provider is one that will create all required resources for a fully functional instance, will start the instance. Waiting for the instance to start is not necessary. If the instance can reach the ```callback_url``` configured in ```garm```, it will update it's own status when it starts running the userdata script.

But aside from creating resources, the ideal external provider is also idempotent, and will clean up after itself in case of failure. If for any reason the executable will fail to create the instance, any dependency that it has created up to the point of failure, should be cleaned up before returning an error code.

At the very least, it must be able to clean up those resources, if it is called with the ```DeleteInstance``` command by ```garm```. Garm will retry creating a failed instance. Before it tries again, it will attempt to run a ```DeleteInstance``` using the ```provider_id``` returned by your executable.

If your executable failed before a ```provider_id``` could be supplied, ```garm``` will send the name of the instance as a ```GARM_INSTANCE_ID``` environment variable.

Your external provider will need to be able to handle both. The instance name generated by ```garm``` will be unique, so it's fairly safe to use when deleting instances.

### CreateInstance inputs

The ```CreateInstance``` command is the only command that needs to handle standard input. Garm will send the runner bootstrap information in stdin. The environment variables set for this command are:

* GARM_PROVIDER_CONFIG_FILE - Config file specific to your executable
* GARM_COMMAND - the command we need to run
* GARM_CONTROLLER_ID - The unique ID of the ```garm``` installation
* GARM_POOL_ID - The unique ID of the pool this node is a part of

The information sent in via standard input is a ```json``` serialized instance of the [BootstrapInstance structure](https://github.com/cloudbase/garm/blob/6b3ea50ca54501595e541adde106703d289bb804/params/params.go#L164-L217)

Here is a sample of that:

  ```json
  {
    "name": "garm-ny9HeeQYw2rl",
    "tools": [
      {
        "os": "osx",
        "architecture": "x64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-osx-x64-2.299.1.tar.gz",
        "filename": "actions-runner-osx-x64-2.299.1.tar.gz",
        "sha256_checksum": "b0128120f2bc48e5f24df513d77d1457ae845a692f60acf3feba63b8d01a8fdc"
      },
      {
        "os": "linux",
        "architecture": "x64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-x64-2.299.1.tar.gz",
        "filename": "actions-runner-linux-x64-2.299.1.tar.gz",
        "sha256_checksum": "147c14700c6cb997421b9a239c012197f11ea9854cd901ee88ead6fe73a72c74"
      },
      {
        "os": "win",
        "architecture": "x64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-win-x64-2.299.1.zip",
        "filename": "actions-runner-win-x64-2.299.1.zip",
        "sha256_checksum": "f7940b16451d6352c38066005f3ee6688b53971fcc20e4726c7907b32bfdf539"
      },
      {
        "os": "linux",
        "architecture": "arm",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-arm-2.299.1.tar.gz",
        "filename": "actions-runner-linux-arm-2.299.1.tar.gz",
        "sha256_checksum": "a4d66a766ff3b9e07e3e068a1d88b04e51c27c9b94ae961717e0a5f9ada998e6"
      },
      {
        "os": "linux",
        "architecture": "arm64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-arm64-2.299.1.tar.gz",
        "filename": "actions-runner-linux-arm64-2.299.1.tar.gz",
        "sha256_checksum": "debe1cc9656963000a4fbdbb004f475ace5b84360ace2f7a191c1ccca6a16c00"
      },
      {
        "os": "osx",
        "architecture": "arm64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-osx-arm64-2.299.1.tar.gz",
        "filename": "actions-runner-osx-arm64-2.299.1.tar.gz",
        "sha256_checksum": "f73849b9a78459d2e08b9d3d2f60464a55920de120e228b0645b01abe68d9072"
      },
      {
        "os": "win",
        "architecture": "arm64",
        "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-win-arm64-2.299.1.zip",
        "filename": "actions-runner-win-arm64-2.299.1.zip",
        "sha256_checksum": "d1a9d8209f03589c8dc05ee17ae8d194756377773a4010683348cdd6eefa2da7"
      }
    ],
    "repo_url": "https://github.com/gabriel-samfira/scripts",
    "callback-url": "https://garm.example.com/api/v1/callbacks",
    "metadata-url": "https://garm.example.com/api/v1/metadata",
    "instance-token": "super secret JWT token",
    "extra_specs": {
      "my_custom_config": "some_value"
    },
    "ca-cert-bundle": null,
    "github-runner-group": "my_group",
    "os_type": "linux",
    "arch": "amd64",
    "flavor": "m1.small",
    "image": "8ed8a690-69b6-49eb-982f-dcb466895e2d",
    "labels": [
      "ubuntu",
      "self-hosted",
      "x64",
      "linux",
      "openstack",
      "runner-controller-id:f9286791-1589-4f39-a106-5b68c2a18af4",
      "runner-pool-id:9dcf590a-1192-4a9c-b3e4-e0902974c2c0"
    ],
    "pool_id": "9dcf590a-1192-4a9c-b3e4-e0902974c2c0"
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

Refer to the OpenStack or Azure providers available in the [providers.d](../contrib/providers.d/) folder. Of particular interest are the [cloudconfig folders](../contrib/providers.d/openstack/cloudconfig/), where the instance user data templates are stored. These templates are used to generate the needed automation for the instances to download the github runner agent, send back status updates (including the final github runner agent ID), and download the github runner registration token from garm.

Examples of external providers written in Go can be found at the followinf locations:

* <https://github.com/cloudbase/garm-provider-azure>
* <https://github.com/cloudbase/garm-provider-openstack>

### CreateInstance outputs

On success, your executable is expected to print to standard output a json that can be deserialized into an ```Instance{}``` structure [defined here](https://github.com/cloudbase/garm/blob/6b3ea50ca54501595e541adde106703d289bb804/params/params.go#L90-L154).

Not all fields are expected to be populated by the provider. The ones that should be set are:

  ```json
  {
    "provider_id": "88818ff3-1fca-4cb5-9b37-84bfc3511ea6",
    "name": "garm-ny9HeeQYw2rl",
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

The environment variables set for this command are:

* GARM_COMMAND
* GARM_CONTROLLER_ID
* GARM_INSTANCE_ID
* GARM_PROVIDER_CONFIG_FILE

This command is not expected to output anything. On success it should simply ```exit 0```.

If the target instance does not exist in the provider, this command is expected to be a no-op.

## GetInstance

The ```GetInstance``` command will return details about the instance, as seen by the provider.

The environment variables set for this command are:

* GARM_COMMAND
* GARM_CONTROLLER_ID
* GARM_INSTANCE_ID
* GARM_PROVIDER_CONFIG_FILE

On success, this command is expected to return a valid ```json``` that can be deserialized into an ```Instance{}``` structure (see CreateInstance). If possible, IP addresses allocated to the VM should be returned in addition to the sample ```json``` printed above.

On failure, this command is expected to return a non-zero exit code.

## ListInstances

The ```ListInstances``` command will print to standard output, a json that is deserializable into an **array** of ```Instance{}```.

The environment variables set for this command are:

* GARM_COMMAND
* GARM_CONTROLLER_ID
* GARM_PROVIDER_CONFIG_FILE
* GARM_POOL_ID

This command must list all instances that have been tagged with the value in ```GARM_POOL_ID```.

On success, a ```json``` is expected on standard output.

On failure, a non-zero exit code is expected.

## RemoveAllInstances

The ```RemoveAllInstances``` operation will remove all resources created in a cloud that have been tagged with the ```GARM_CONTROLLER_ID```. External providers should tag all resources they create with the garm controller ID. That tag can then be used to identify all resources when attempting to delete all instances.

The environment variables set for this command are:

* GARM_COMMAND
* GARM_PROVIDER_CONFIG_FILE
* GARM_CONTROLLER_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.

Note: This command is currently not used by garm.

## Start

The ```Start``` operation will start the virtual machine in the selected cloud.

The environment variables set for this command are:

* GARM_COMMAND
* GARM_CONTROLLER_ID
* GARM_PROVIDER_CONFIG_FILE
* GARM_INSTANCE_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.

## Stop

NOTE: This operation is currently not use by ```garm```, but should be implemented.

The ```Stop``` operation will stop the virtual machine in the selected cloud.

Available environment variables:

* GARM_COMMAND
* GARM_CONTROLLER_ID
* GARM_PROVIDER_CONFIG_FILE
* GARM_INSTANCE_ID

On success, no output is expected.

On failure, a non-zero exit code is expected.
