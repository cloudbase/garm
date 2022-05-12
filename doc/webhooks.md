# Webhooks

Garm is designed to auto-scale github runners based on a few simple rules:

  * A minimum idle runner count can be set for a pool. Garm will attempt to maintain that minimum of idle runners, ready to be used by your workflows.
  * A maximum number of runners for a pool. This is a hard limit of runners a pool will create, regardless of minimum idle runners.
  * When a runner is scheduled by github, ```garm``` will automatically spin up a new runner to replace it, obeying the maximum hard limit defined.

To achieve this, ```garm``` relies on [GitHub Webhooks](https://docs.github.com/en/developers/webhooks-and-events/webhooks/about-webhooks). Webhooks allow ```garm``` to react to workflow events from your repository or organization.

In your repository or organization, navigate to ```Settings --> Webhooks```. In the ```Payload URL``` field, enter the URL to the ```garm``` webhook endpoint. The ```garm``` API endpoint for webhooks is:

```
POST /webhooks
```

If ```garm``` is running on a server under the domain ```garm.example.com```, then that field should be set to ```https://garm.example.com/webhooks```.

In the webhook configuration page under ```Content type``` you will need to select ```application/json```, set the proper webhook URL and, really important, **make sure you configure a webhook secret**. Garm will authenticate the payloads to make sure they are coming from GitHub.

The webhook secret must be secure. Use something like this to generate one:

```bash
gabriel@rossak:~$ function generate_secret () { 
    tr -dc 'a-zA-Z0-9!@#$%^&*()_+?><~\`;' < /dev/urandom | head -c 64;
    echo ''
}

gabriel@rossak:~$ generate_secret
9Q<fVm5dtRhUIJ>*nsr*S54g0imK64(!2$Ns6C!~VsH(p)cFj+AMLug%LM!R%FOQ
```

Next, you can choose which events GitHub should send to ```garm``` via webhooks. Click on ```Let me select individual events``` and select ```Workflow jobs``` (should be at the bottom). You can send everything if you want, but any events ```garm``` doesn't care about will simply be ignored.