# Webhooks

GARM uses GitHub/Gitea webhooks to learn when workflow jobs are queued, so it can spin up runners on demand. GARM can manage webhooks automatically for repositories and organizations, or you can set them up manually.

## Automatic webhook management

When adding a repository or organization, pass `--install-webhook` and `--random-webhook-secret`:

```bash
garm-cli repo add \
  --owner your-org \
  --name your-repo \
  --credentials my-pat \
  --random-webhook-secret \
  --install-webhook
```

This requires the PAT or App to have `admin:repo_hook` (or `admin:org_hook`) permissions.

GARM uses the **Controller Webhook URL** (unique per GARM installation):

```bash
garm-cli controller show
```

```
+------------------------+-----------------------------------------------------------------------+
| Controller Webhook URL | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3 |
+------------------------+-----------------------------------------------------------------------+
```

## Manual webhook setup

If you prefer to manage webhooks yourself:

1. Go to your repository or organization **Settings > Webhooks > Add webhook**
2. **Payload URL:** Use the Controller Webhook URL from `garm-cli controller show`
3. **Content type:** Select `application/json`
4. **Secret:** Use a strong random string (64+ characters). You'll need this when adding the entity to GARM.

   ```bash
   tr -dc 'a-zA-Z0-9!@#$%^&*()_+' < /dev/urandom | head -c 64; echo
   ```

5. **Events:** Click "Let me select individual events" and select only **Workflow jobs**
6. **SSL verification:** Enable for production (use a proper TLS certificate)
7. Click **Add webhook**

Then add the entity to GARM with the same secret:

```bash
garm-cli repo add \
  --owner your-org \
  --name your-repo \
  --credentials my-pat \
  --webhook-secret "the-secret-you-used-in-github"
```

## Enterprise webhooks

Enterprise webhooks must always be set up manually. GARM does not manage enterprise-level webhooks:

```bash
garm-cli enterprise add \
  --name enterprise-slug \
  --credentials my-enterprise-pat \
  --webhook-secret "your-secret"
```

Then configure the webhook in GitHub Enterprise Settings using the Controller Webhook URL.

## Troubleshooting

### Webhook not receiving events

- Verify the Webhook URL is reachable from GitHub (must be internet-accessible for github.com)
- Check for a green checkmark next to the webhook in GitHub settings
- Ensure you selected the "Workflow jobs" event
- Check GARM logs: `garm-cli debug-log`

### Idle runners not picking up jobs

- Check that pool tags match the workflow's `runs-on` labels. Runners that are already online will only pick up jobs whose labels match.

### GARM not scaling up new runners

- Verify the webhook secret matches between GitHub and GARM
- Check that pool tags match the workflow's `runs-on` labels
- Check recorded jobs: `garm-cli job list`
- Review the job age backoff: `garm-cli controller show` (default: 30 seconds)

### Using HTTPS

For production, use HTTPS with a valid certificate. [Let's Encrypt](https://letsencrypt.org/) provides free certificates. If using a self-signed certificate, you can disable SSL verification in the GitHub webhook settings, but this is not recommended for production.
