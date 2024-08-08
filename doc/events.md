# GARM database events

Starting with GARM version `v0.1.5`, we now have a new websocket endpoint that allows us to subscribe to some events that are emited by the database watcher. Whenever a database entity is created, updated or deleted, the database watcher will notify all interested consumers that an event has occured and as part of that event, we get a copy of the database entity that was affected.

For example, if a new runner is created, the watcher will emit a `Create` event for the `Instances` entity and in the `Payload` field, we will have a copy of the `Instance` entity that was created. Internally, this will be a golang struct, but when exported via the websocket endpoint, it will be a JSON object, with all sensitive info (passwords, keys, secrets in general) stripped out.

This document will focus on the websocket endpoint and the events that are exported by it.

# Entities and operations

Virtually all database entities are exposed through the events endpoint. These entities are defined in the [database common package](https://github.com/cloudbase/garm/blob/56b0e6065a993fd89c74a8b4ab7de3487544e4e0/database/common/watcher.go#L12-L21). Each of the entity types represents a database table in GARM.

Those entities are:

* `repository` - represents a repository in the database
* `organization` - represents an organization in the database
* `enterprise` - represents an enterprise in the database
* `pool` - represents a pool in the database
* `user` - represents a user in the database. Currently GARM is not multi tenant so we just have the "admin" user
* `instance` - represents a runner instance in the database
* `job` - represents a recorded github workflow job in the database
* `controller` - represents a controller in the database. This is the GARM controller.
* `github_credentials` - represents a github credential in the database (PAT, Apps, etc). No sensitive info (token, keys, etc) is ever returned by the events endpoint.
* `github_endpoint` - represents a github endpoint in the database. This holds the github.com default endpoint and any GHES you may add.

The operations hooked up to the events endpoint and the databse wather are:

* `create` - emitted when a new entity is created
* `update` - emitted when an entity is updated
* `delete` - emitted when an entity is deleted

# Event structure

The event structure is defined in the [database common package](https://github.com/cloudbase/garm/blob/56b0e6065a993fd89c74a8b4ab7de3487544e4e0/database/common/watcher.go#L30-L34). The structure for a change payload is marshaled into a JSON object as follows:

```json
{
    "entity-type": "repository",
    "operation": "create"
    "payload": [object]
}
```

Where the `payload` will be a JSON representation of one of the entities defined above. Essentially, you can expect to receive a JSON identical to the one you would get if you made an API call to the GARM REST API for that particular entity.

Note that in some cases, the `delete` operation will return the full object prior to the deletion of the entity, while others will only ever return the `ID` of the entity. This will probably be changed in future releases to only return the `ID` in case of a `delete` operation, for all entities. You should operate under the assumption that in the future, delete operations will only return the `ID` of the entity.

# Subscribing to events

By default the events endpoint returns no events. All events are filtered by default. To start receiving events, you need to emit a message on the websocket connection indicating the entities and/or operations you're interested in.

This gives you the option to get fine grained control over what you receive at any given point in time. Of course, you can opt to receive everything and deal with the potential deluge (depends on how busy your GARM instance is) on your own.

## The filter message

The filter is defined as a JSON that you write over the websocket connections. That JSON must adhere to the following schema:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://github.com/cloudbase/garm/apiserver/events/options",
  "$ref": "#/$defs/Options",
  "$defs": {
    "Filter": {
      "properties": {
        "operations": {
          "items": {
            "type": "string",
            "enum": [
              "create",
              "update",
              "delete"
            ]
          },
          "type": "array",
          "title": "operations",
          "description": "A list of operations to filter on"
        },
        "entity-type": {
          "type": "string",
          "enum": [
            "repository",
            "organization",
            "enterprise",
            "pool",
            "user",
            "instance",
            "job",
            "controller",
            "github_credentials",
            "github_endpoint"
          ],
          "title": "entity type",
          "description": "The type of entity to filter on",
          "default": "repository"
        }
      },
      "additionalProperties": false,
      "type": "object"
    },
    "Options": {
      "properties": {
        "send-everything": {
          "type": "boolean",
          "title": "send everything",
          "default": false
        },
        "filters": {
          "items": {
            "$ref": "#/$defs/Filter"
          },
          "type": "array",
          "title": "filters",
          "description": "A list of filters to apply to the events. This is ignored when send-everything is true"
        }
      },
      "additionalProperties": false,
      "type": "object"
    }
  }
}
```

But I realize a JSON schema is not the best way to explain how to use the filter. The following examples should give you a better idea of how to use the filter.

### Example 1: Send all events

```json
{
  "send-everything": true
}
```

### Example 2: Send only `create` events for `repository` entities

```json
{
  "send-everything": false,
  "filters": [
    {
      "entity-type": "repository",
      "operations": ["create"]
    }
  ]
}
```

### Example 3: Send `create` and `update` for repositories and `delete` for instances

```json
{
  "send-everything": false,
  "filters": [
    {
      "entity-type": "repository",
      "operations": ["create", "update"]
    },
    {
      "entity-type": "instance",
      "operations": ["delete"]
    }
  ]
}
```

## Connecting to the events endpoint

You can use any websocket client, written in any programming language to interact with the events endpoint. In the following exmple I'll show you how to do it from go.

Before we start, we'll need a JWT token to access the events endpoint. Normally, if you use the CLI, you should have it in your `~/.local/share/garm-cli` folder. But if you know your username and password, we can fetch a fresh one using `curl`:

```bash
# Read the password from the terminal
read -s PASSWD

# Get the token
curl -s -X POST -d '{"username": "admin", "password": "'$PASSWD'"}' \
  https://garm.example.com/api/v1/auth/login | jq -r .token
```

Save the token, we'll need it for later.

Now, let's write a simple go program that connects to the events endpoint and subscribes to all events. We'll use the reader that was added to [`garm-provider-common`](https://github.com/cloudbase/garm-provider-common) in version `v0.1.3`, to make this easier:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	garmWs "github.com/cloudbase/garm-provider-common/util/websocket"
	"github.com/gorilla/websocket"
)

// List of signals to interrupt the program
var signals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

// printToConsoleHandler is a simple function that prints the message to the console.
// In a real world implementation, you can use this function to decide how to properly
// handle the events.
func printToConsoleHandler(_ int, msg []byte) error {
	fmt.Println(string(msg))
	return nil
}

func main() {
	// Set up the context to listen for signals.
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()

	// This is the JWT token you got from the curl command above.
	token := "superSecretJWTToken"
	// The base URL of your GARM server
	baseURL := "https://garm.example.com"
	// This is the path to the events endpoint
	pth := "/api/v1/ws/events"

	// Instantiate the websocket reader
	reader, err := garmWs.NewReader(ctx, baseURL, pth, token, printToConsoleHandler)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Start the loop.
	if err := reader.Start(); err != nil {
		fmt.Println(err)
		return
	}

	// Set the filter to receive all events. You can use a more fine grained filter if you wish.
	reader.WriteMessage(websocket.TextMessage, []byte(`{"send-everything":true}`))

	fmt.Println("Listening for events. Press Ctrl+C to stop.")
	// Wait for the context to be done.
	<-ctx.Done()
}
```

If you run this program and change something in the GARM database, you should see the event being printed to the console:

```bash
gabriel@rossak:/tmp/ex$ go run ./main.go
{"entity-type":"pool","operation":"update","payload":{"runner_prefix":"garm","id":"8ec34c1f-b053-4a5d-80d6-40afdfb389f9","provider_name":"lxd","max_runners":10,"min_idle_runners":0,"image":"ubuntu:22.04","flavor":"default","os_type":"linux","os_arch":"amd64","tags":[{"id":"76781c93-e354-402e-907a-785caab36207","name":"self-hosted"},{"id":"2ff4a89e-e3b4-4e78-b977-6c21e83cca3d","name":"x64"},{"id":"5b3ffec6-0402-4322-b2a9-fa7f692bbc00","name":"Linux"},{"id":"e95e106d-1a3d-11ee-bd1d-00163e1f621a","name":"ubuntu"},{"id":"3b54ae6c-5e9b-4a81-8e6c-0f78a7b37b04","name":"repo"}],"enabled":true,"instances":[],"repo_id":"70227434-e7c0-4db1-8c17-e9ae3683f61e","repo_name":"gsamfira/scripts","runner_bootstrap_timeout":20,"extra_specs":{"disable_updates":true,"enable_boot_debug":true},"github-runner-group":"","priority":10}}
```

In the above example, you can see an `update` event on a `pool` entity. The `payload` field contains the full, updated `pool` entity.
