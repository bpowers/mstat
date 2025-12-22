`mstat` -- measure memory usage of a program over time
====================================================

This tool runs on Linux, taking advantage of the `cgroups` kernel API
(also used by container infrastructure like Docker) to record memory
usage of a set of processes over time.  Because `mstat` builds on
cgroups, we automatically track memory usage of any child-processes
spawned by the original program.

**Supports both cgroups v1 (legacy) and cgroups v2 (unified hierarchy).**
The tool auto-detects which cgroup version is available on your system.

Additionally, on cgroups v1 systems, the [Memory
API](https://godoc.org/github.com/containerd/cgroups) we are using
gives us detailed information about userspace memory usage, as well as
kernel memory allocated on behalf of the program (such as memory used
to manage a process's page tables). Note: in cgroups v2, the separate
kernel memory controller was removed - kernel memory is included in the
total usage but cannot be queried separately.

This tool only runs on Linux, and requires being installed set-UID.
Build it the normal way:

    $ git clone https://github.com/bpowers/mstat
    $ cd mstat
    $ make; sudo make install

Then, use it to measure memory usage over time (freq specifies the
sampling frequency in Hz, bump it up for short-lived programs or fine
grained reporting):

    $ mstat -o data/mem.tsv -freq 59 -- ./test

And there is even a handy flag to modify the environment:

    $ mstat -o data/mem.tsv -freq 59 -env LD_PRELOAD=libawesome.so -- ./test

Use `-v` for verbose mode to see which cgroups version is being used:

    $ mstat -v -o data/mem.tsv -- ./test
    mstat: using cgroups mode: unified (v2)

## Requirements

- Go 1.24 or later
- Linux with cgroups v1 or v2 support
- Root privileges (or setuid binary) to create cgroups
