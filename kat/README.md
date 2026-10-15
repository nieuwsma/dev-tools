<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# kat: which local Kind apps are expensive

`kat` is the installed name of kind-app-top. It is a read-only `top`-style view of application resource usage across local Kind clusters.

```text
cluster -> application namespace -> owning workload family
```

Numbered `*-slot-NN` controllers collapse into one family. Metrics Server is not required. The command reads kubelet summary stats through the Kubernetes API. It does not install anything in a cluster.

## Install

From this directory, or from the repo:

```sh
./kat/install.sh
kat --help
```

Installation builds one executable with the local Go toolchain and copies it to `~/.local/bin/kat`. The source checkout is not needed afterward. Changed existing executables are backed up beside the installed file. Repeat installations of the same binary do nothing.

Requirements: macOS, Go, `kubectl`, and kubeconfig contexts named `kind-*` unless you pass `--contexts`. The kubeconfig identity needs read access to pods, workload controllers, nodes, and the node `proxy` subresource.

## Use

```sh
kat
kat --once --no-color
kat --json --filter rack
kat --sort cpu --level namespaces
```

In the live view:

| Key | Action |
| --- | --- |
| `[` / `]` | Show one fewer or one more hierarchy level |
| `m` / `c` / `n` | Sort by memory, CPU, or name |
| `q` | Quit |

`MEM REQ` and `MEM LIM` are the sums of container memory requests and limits. `ΔMEM` is the working-set change since the previous refresh. The first sample is `0`.

`--filter` keeps namespaces whose names contain the text, and otherwise keeps only the matching apps inside a namespace. `--json` prints one snapshot and exits. `--sort` accepts `memory`, `cpu`, or `name`.

Cluster CPU and memory include Kubernetes and operating-system overhead. The `TOTAL` row sums usage and uses the largest cluster-visible host capacity, because local Kind clusters usually share one runtime virtual machine.

## Verification

```sh
go test ./...
```

The tests do not contact a cluster.
