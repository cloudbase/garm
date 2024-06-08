# ExtraSpecs

ExtraSpecs is an opaque raw json that gets sent to the provider as part of the bootstrap params for instances. It can contain any kind of data needed by providers. The contents of this field means nothing to garm itself. We don't act on the information in this field at all. We only validate that it's a proper json.

However, during the installation phase of the runners, GARM providers can leverage the information set in this field to augment the process in many ways. This can be used for anything ranging from overriding provider config values, to supplying a different runner install template, to passing in information that is relevant only to specific providers.

For example, the [external OpenStack provider](https://github.com/cloudbase/garm-provider-openstack) uses this to [override](https://github.com/cloudbase/garm-provider-openstack#tweaking-the-provider) things like `security groups`, `storage backends`, `network ids`, etc.

