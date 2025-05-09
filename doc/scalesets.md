# Scale Sets

<!-- TOC -->

- [Scale Sets](#scale-sets)
    - [Create a new scale set](#create-a-new-scale-set)
    - [Scale Set vs Pool](#scale-set-vs-pool)

<!-- /TOC -->

GARM supports [scale sets](https://docs.github.com/en/actions/hosting-your-own-runners/managing-self-hosted-runners-with-actions-runner-controller/deploying-runner-scale-sets-with-actions-runner-controller). This new mode of operation was added by GitHub to enable more efficient scheduling of runners using their own ARC (Actions Runner Controller) project. The APIs for enabling scale sets are not yet public and the scale set functionlity itself is not terribly well documented outside the context of ARC, but it can be implemented in third party auto scalers.

In this document we will focus on how scale sets work, how they are different than pools and how to manage them.

We'll start with detailing how to create a scale set.

## Create a new scale set

Creating a scale set is identical to [creating a pool](/doc/using_garm.md#creating-a-runner-pool), but instead of adding labels to a scale set, it takes a name. We'll assume you already have a provider enabled and you have added a repo, org or enterprise to GARM.

```bash
ubuntu@garm:~$ garm-cli repo ls
+--------------------------------------+-----------+--------------+------------+------------------+--------------------+------------------+
| ID                                   | OWNER     | NAME         | ENDPOINT   | CREDENTIALS NAME | POOL BALANCER TYPE | POOL MGR RUNNING |
+--------------------------------------+-----------+--------------+------------+------------------+--------------------+------------------+
| 84a5e82f-7ab1-427f-8ee0-4569b922296c | gsamfira  | garm-testing | github.com | gabriel-samfira  | roundrobin         | true             |
+--------------------------------------+-----------+--------------+------------+------------------+--------------------+------------------+
```

List providers:

```bash
ubuntu@garm:~$ garm-cli provider list
+--------------+---------------------------------+----------+
| NAME         | DESCRIPTION                     | TYPE     |
+--------------+---------------------------------+----------+
| incus        | Incus external provider         | external |
+--------------+---------------------------------+----------+
| azure        | azure provider                  | external |
+--------------+---------------------------------+----------+
| aws_ec2      | Amazon EC2 provider             | external |
+--------------+---------------------------------+----------+
```

Create a new scale set:

```bash
garm-cli scaleset add \
    --repo  84a5e82f-7ab1-427f-8ee0-4569b922296c \
    --provider-name incus \
    --image ubuntu:22.04 \
    --name garm-scale-set \
    --flavor default \
    --enabled true \
    --min-idle-runners=0 \
    --max-runners=20
+--------------------------+-----------------------+
| FIELD                    | VALUE                 |
+--------------------------+-----------------------+
| ID                       | 8                     |
| Scale Set ID             | 14                    |
| Scale Name               | garm-scale-set        |
| Provider Name            | incus                 |
| Image                    | ubuntu:22.04          |
| Flavor                   | default               |
| OS Type                  | linux                 |
| OS Architecture          | amd64                 |
| Max Runners              | 20                    |
| Min Idle Runners         | 0                     |
| Runner Bootstrap Timeout | 20                    |
| Belongs to               | gsamfira/garm-testing |
| Level                    | repo                  |
| Enabled                  | true                  |
| Runner Prefix            | garm                  |
| Extra specs              |                       |
| GitHub Runner Group      | Default               |
+--------------------------+-----------------------+
```

That's it. You now have a scale set created, ready to accept jobs.

## Scale Set vs Pool

Scale sets are a new way of managing runners. They were introduced by GitHub to enable more efficient scheduling of runners using their own Actions Runner Controller (ARC) project. Scale sets are meant to reduce API calls, improve reliability of message deliveries and improve efficiency of runner management. While webhooks work great most of the time, under heavy load, they may not fire or they may fire while the auto scaler is offline. If webhooks are fired while GARM is down, we will never know about those jobs unless we query the current workflow runs.

Listing workflow runs is not feisable for orgs or enterprises, as that would mean listing all repos withing an org then for each repository, listing all workflow runs. This gets worse for enterprises. Scale sets on the other hand allows GARM to subscribe to a message queue and get messages just for that scale set over HTTP long poll.

Advantages of scale sets over pools:

* No more need to install a webhook, reducing your security footprint.
* Scheduling is done by GitHub. GARM receives runner requests from GitHub and GARM can choose to acquire those jobs or leave them for some other scaler.
* Easier use of runner groups. While GARM supports runner groups, github currently [does not send the group name](https://github.com/orgs/community/discussions/158000) as part of webhooks in `queued` state. This prevents GARM (or any other auto scaler) to efficiently schedule runners to pools that have runner groups set. But given that in the case of scale sets, GitHub schedules the runners to the scaleset itself, we can efficiently create runners in certain runner groups.
* scale set names must be unique within a runner group