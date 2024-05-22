# Labels and Tags

Github Runners can be tagged with labels. These labels can be used to restrict the jobs that can run on a runner. For example, you can have a runner with the label `linux` and another with the label `windows`. You can then restrict a job to run only on a runner with the label `linux`.

Whenever a new runner register themselves on Github, the runner knows its own labels as the labels are defined in the pool specification as tags.

Beside the custom labels, Github also has some predefined labels that are appended by the runner script per default.
These are: 
```yaml
[ 'self-hosted', '$OS_TYPE', '$OS_ARCH' ]
```

With Version `v0.1.2` of `garm-provider-common`, the runner script will register themselves with a new command line flag, called `--no-default-labels`. If this flag is set, the runner will not append any default label.

As all labels can be defined in the pool specification, it's still possible to add the default labels manually.
