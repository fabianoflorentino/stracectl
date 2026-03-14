// +build ignore
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct syscall_event {
    __u32 pid;
    __u32 syscall_nr;
    __s64 ret;
    __u64 enter_ns;
    __u64 exit_ns;
    __u64 args[6];
};

struct enter_data {
    __u64 ts;
    __u64 syscall_nr;
    __u64 args[6];
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 128);
    __type(key, __u32);
    __type(value, struct enter_data);
} enter_data_map SEC(".maps");

/*
target_pid[0] = PID to trace
0 = trace everything
*/
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u32);
} target_pid SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 12);
} events SEC(".maps");

static __always_inline int should_trace(__u32 pid) {
    __u32 key = 0;
    __u32 *target = bpf_map_lookup_elem(&target_pid, &key);

    if (!target || *target == 0)
        return 1;

    if (pid == *target)
        return 1;

    return 0;
}

SEC("raw_tracepoint/sys_enter")
int sys_enter(struct bpf_raw_tracepoint_args *ctx) {
    __u64 id  = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    __u32 tid = (__u32)id;

    if (pid == 0)
        return 0;

    if (!should_trace(pid))
        return 0;

    struct pt_regs *regs = (struct pt_regs *)ctx->args[0];
    __u64 nr = ctx->args[1];

    struct enter_data d = {};
    d.ts         = bpf_ktime_get_ns();
    d.syscall_nr = nr;

    bpf_probe_read_kernel(&d.args[0], sizeof(__u64), &regs->di);
    bpf_probe_read_kernel(&d.args[1], sizeof(__u64), &regs->si);
    bpf_probe_read_kernel(&d.args[2], sizeof(__u64), &regs->dx);
    bpf_probe_read_kernel(&d.args[3], sizeof(__u64), &regs->r10);
    bpf_probe_read_kernel(&d.args[4], sizeof(__u64), &regs->r8);
    bpf_probe_read_kernel(&d.args[5], sizeof(__u64), &regs->r9);

    bpf_map_update_elem(&enter_data_map, &tid, &d, BPF_ANY);

    return 0;
}

SEC("raw_tracepoint/sys_exit")
int sys_exit(struct bpf_raw_tracepoint_args *ctx) {
    __u64 id  = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    __u32 tid = (__u32)id;

    struct enter_data *d = bpf_map_lookup_elem(&enter_data_map, &tid);
    if (!d)
        return 0;

    struct syscall_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        bpf_map_delete_elem(&enter_data_map, &tid);
        return 0;
    }

    e->pid        = pid;
    e->syscall_nr = (__u32)d->syscall_nr;
    e->ret        = ctx->args[1];
    e->enter_ns   = d->ts;
    e->exit_ns    = bpf_ktime_get_ns();

    e->args[0] = d->args[0];
    e->args[1] = d->args[1];
    e->args[2] = d->args[2];
    e->args[3] = d->args[3];
    e->args[4] = d->args[4];
    e->args[5] = d->args[5];

    bpf_ringbuf_submit(e, 0);

    bpf_map_delete_elem(&enter_data_map, &tid);

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
