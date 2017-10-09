`mstat` -- measure memory usage of a program over time
====================================================

This tool runs on Linux, taking advantage of the `cgroups` kernel API
(also used by container infrastructure like Docker) to record memory
usage of a set of processes over time.  Because `mstat` builds on
`groups`, we automatically track memory usage of any child-processes
spawned by the original program.

Additionally, the [Memory
API](https://godoc.org/github.com/containerd/cgroups#MemoryStat) we
are using not only gives us detailed information about userspace
memory usage, but _also_ about kernel memory allocated on behalf of
the program.  (such as memory used to mange a process's page tables).

This tool only runs on Linux, and requires being installed installed
set-UID.  Build it the normal way:

    $ git clone https://github.com/bpowers/mstat
    $ cd mstat
    $ make; sudo make install

Then, use it to measure memory usage over time:

    $ mstat -o data/mem.tsv -freq 59 -- ./test

And there is even a handy flag to modify the environment:

    $ mstat -o data/mem.tsv -freq 59 -env LD_PRELOAD=libawesome.so -- ./test
