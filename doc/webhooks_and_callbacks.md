# Webhooks

Garm is designed to auto-scale github runners based on a few simple rules:

* A minimum idle runner count can be set for a pool. Garm will attempt to maintain that minimum of idle runners, ready to be used by your workflows.
* A maximum number of runners for a pool. This is a hard limit of runners a pool will create, regardless of minimum idle runners.
* When a runner is scheduled by github, ```garm``` will automatically spin up a new runner to replace it, obeying the maximum hard limit defined.

To achieve this, ```garm``` relies on [GitHub Webhooks](https://docs.github.com/en/developers/webhooks-and-events/webhooks/about-webhooks). Webhooks allow ```garm``` to react to workflow events from your repository or organization.

In your repository or organization, navigate to ```Settings --> Webhooks```. In the ```Payload URL``` field, enter the URL to the ```garm``` webhook endpoint. The ```garm``` API endpoint for webhooks is:

  ```txt
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

## The callback_url option

Your runners will call back home with status updates as they install. Once they are set up, they will also send the GitHub agent ID they were allocated. You will need to configure the ```callback_url``` option in the ```garm``` server config. This URL needs to point to the following API endpoint:

  ```txt
  POST /api/v1/callbacks/status
  ```

Example of a runner sending status updates:

  ```bash
  garm-cli runner show garm-f5227755-129d-4e2d-b306-377a8f3a5dfe
  +-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
  | FIELD           | VALUE                                                                                                                                            |
  +-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
  | ID              | 1afb407b-e9f7-4d75-a410-fc4a8c2dbe6c                                                                                                             |
  | Provider ID     | garm-f5227755-129d-4e2d-b306-377a8f3a5dfe                                                                                                        |
  | Name            | garm-f5227755-129d-4e2d-b306-377a8f3a5dfe                                                                                                        |
  | OS Type         | linux                                                                                                                                            |
  | OS Architecture | amd64                                                                                                                                            |
  | OS Name         | ubuntu                                                                                                                                           |
  | OS Version      | focal                                                                                                                                            |
  | Status          | running                                                                                                                                          |
  | Runner Status   | idle                                                                                                                                             |
  | Pool ID         | 98f438b9-5549-4eaf-9bb7-1781533a455d                                                                                                             |
  | Status Updates  | 2022-05-05T11:32:41: downloading tools from https://github.com/actions/runner/releases/download/v2.290.1/actions-runner-linux-x64-2.290.1.tar.gz |
  |                 | 2022-05-05T11:32:43: extracting runner                                                                                                           |
  |                 | 2022-05-05T11:32:47: installing dependencies                                                                                                     |
  |                 | 2022-05-05T11:32:55: configuring runner                                                                                                          |
  |                 | 2022-05-05T11:32:59: installing runner service                                                                                                   |
  |                 | 2022-05-05T11:33:00: starting service                                                                                                            |
  |                 | 2022-05-05T11:33:00: runner successfully installed                                                                                               |
  +-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
  ```

This URL must be set and must be accessible by the instance. If you wish to restrict access to it, a reverse proxy can be configured to accept requests only from networks in which the runners ```garm``` manages will be spun up. This URL doesn't need to be globally accessible, it just needs to be accessible by the instances.

For example, in a scenario where you expose the API endpoint directly, this setting could look like the following:

  ```toml
  callback_url = "https://garm.example.com/api/v1/callbacks/status"
  ```

Authentication is done using a short-lived JWT token, that gets generated for a particular instance that we are spinning up. That JWT token grants access to the instance to only update it's own status and to fetch metadata for itself. No other API endpoints will work with that JWT token. The validity of the token is equal to the pool bootstrap timeout value (default 20 minutes) plus the garm polling interval (5 minutes).

There is a sample ```nginx``` config [in the testdata folder](/testdata/nginx-server.conf). Feel free to customize it whichever way you see fit.

## The metadata_url option

The metadata URL is the base URL for any information an instance may need to fetch in order to finish setting itself up. As this URL may be placed behind a reverse proxy, you'll need to configure it in the ```garm``` config file. Ultimately this URL will need to point to the following ```garm``` API endpoint:

  ```bash
  GET /api/v1/metadata
  ```

This URL needs to be accessible only by the instances ```garm``` sets up. This URL will not be used by anyone else. To configure it in ```garm``` add the following line in the ```[default]``` section of your ```garm``` config:

  ```toml
  metadata_url = "https://garm.example.com/api/v1/metadata"
  ```
