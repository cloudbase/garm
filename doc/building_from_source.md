# Building GARM from source

The procedure is simple. You will need to gave [go](https://golang.org/) installed as well as `make`.

First, clone the repository:

```bash
git clone https://github.com/cloudbase/garm
```

Then build garm:

```bash
make
```

You should now have both `garm` and `garm-cli` available in the `./bin` folder.

If you have docker/podman installed, you can also build a static binary against `musl`:

```bash
make build-static
```

This command will also build for both AMD64 and ARM64. Resulting binaries will be in the `./bin` folder.