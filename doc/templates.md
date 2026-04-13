# Runner Install Templates

Templates control the scripts that set up runners on new instances. GARM ships with built-in system templates for GitHub and Gitea on both Linux and Windows. You can create custom templates to modify the bootstrap process.

## Listing templates

```bash
# All templates
garm-cli template list

# Filter by forge type and OS
garm-cli template list --forge-type github --os-type linux
```

## Viewing a template

```bash
garm-cli template show <TEMPLATE_NAME_OR_ID>
```

## Creating a custom template

Write your template script to a file, then create it in GARM:

```bash
garm-cli template create \
  --name my-custom-template \
  --description "Custom runner setup with extra packages" \
  --forge-type github \
  --os-type linux \
  --path /path/to/my-template.sh
```

## Cloning an existing template

Start from a built-in template and modify it:

```bash
# Copy the system template to a new name
garm-cli template copy system-github-linux my-custom-template

# Edit the template in the built-in TUI editor
garm-cli template edit my-custom-template
```

You can also download a template to a file for inspection:

```bash
garm-cli template download my-custom-template --path /tmp/my-template.sh
```

## Using a template with a pool

Specify the template when creating or updating a pool:

```bash
# At creation
garm-cli pool add \
  --runner-install-template my-custom-template \
  --repo <REPO_ID> \
  --provider-name lxd_local \
  --image ubuntu:22.04 \
  --flavor default \
  --tags ubuntu

# Or update an existing pool
garm-cli pool update <POOL_ID> --runner-install-template my-custom-template
```

## Restoring system templates

If you've modified a system template and want to restore the defaults:

```bash
garm-cli template restore
```

## Deleting a template

> [!IMPORTANT]
> You cannot delete a template that is in use by a pool or scale set.

```bash
garm-cli template delete <TEMPLATE_NAME_OR_ID>
```
