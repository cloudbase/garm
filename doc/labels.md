# Labels and Tags

Github Runners can be tagged with labels. These labels can be used to restrict the jobs that can run on a runner. For example, you can have a runner with the label `linux` and another with the label `windows`. You can then restrict a job to run only on a runner with the label `linux`.

Whenever a new runner register themselves on Github, the runner knows its own labels as the labels are defined in the pool specification as tags.

Before version 2.305.0 of the runner and before JIT runners were introduced, the runner registration process would append some default labels to the runner. These labels are:

```yaml
[ 'self-hosted', '$OS_TYPE', '$OS_ARCH' ]
```

This made scheduling and using runners a bit awkward in some situations. For example, in large organizations with many teams, often times workflows would simply target the `self-hosted` label. This would match all runners regardless of any other custom labels. This had the side effect that workflows would potentially use expensive runners for simple jobs or would select low resource runners for tasks that would require a lot of resources.

Version 2.305.0 of the runner introduced the `--no-default-labels` flag when registering the runner. When JIT is not available (GHES version < 3.10), GARM will now register the runner with the `--no-default-labels` flag. If you still need the default labels, you can still add them when creating the pool as part of the `--tags` command line option.
