# GARM database events

Starting with GARM version `v0.1.5`, we now have a new websocket endpoint that allows us to subscribe to some events that are emited by the database watcher. Whenever a database entity is created, updated or deleted, the database watcher will notify all interested consumers that an event has occured, and as part of that event, the database entity that was affected.

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

Note that in some cases, the `delete` operation will return the full object prior to the deletion of the entity, while others will only ever return the `ID` of the entity. This will probably be changed in future releases to only return the `ID` in case of a `delete` operation, for all entities.

# Subscribing to events

By default the events endpoint returns no events. All events are filtered by default. To start receiving events, you need to emit a message on the websocket connection indicating the entities and/or operations you're interested in.

This gives you the option to get fine grained control over what you receive at any given point in time. Of course, you can opt to receive everything and deal with the potential deluge (depends on how busy your GARM instance is) on your own.