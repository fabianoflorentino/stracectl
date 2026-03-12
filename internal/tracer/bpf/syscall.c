// +build ignore
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Struct enviada ao user-space via ring buffer
struct syscall_event {
    __u32 pid;
    __u32 syscall_nr;
    __s64 ret;
    __u64 enter_ns;   // bpf_ktime_get_ns() no entry
    __u64 exit_ns;    // bpf_ktime_get_ns() no exit
    __u64 args[6];    // argumentos do syscall
};

// Mapa temporário: tid -> enter_ns (guarda o timestamp de entrada)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);
    __type(value, __u64);
} enter_times SEC(".maps");

// Ring buffer para enviar eventos ao user-space
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16 MB
} events SEC(".maps");

SEC("raw_tracepoint/sys_enter")
int sys_enter(struct bpf_raw_tracepoint_args *ctx) {
    __u64 id  = bpf_get_current_pid_tgid();
    __u32 tid = (__u32)id;
    __u64 ts  = bpf_ktime_get_ns();
    bpf_map_update_elem(&enter_times, &tid, &ts, BPF_ANY);
    return 0;
}

SEC("raw_tracepoint/sys_exit")
int sys_exit(struct bpf_raw_tracepoint_args *ctx) {
    __u64 id  = bpf_get_current_pid_tgid();
    __u32 pid = id >> 32;
    __u32 tid = (__u32)id;

    __u64 *enter_ns = bpf_map_lookup_elem(&enter_times, &tid);
    if (!enter_ns) return 0;

    struct syscall_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    e->pid        = pid;
    e->syscall_nr = ((__u32)ctx->args[1]); // nr no sys_exit
    e->ret        = ctx->args[0];
    e->enter_ns   = *enter_ns;
    e->exit_ns    = bpf_ktime_get_ns();
    bpf_ringbuf_submit(e, 0);

    bpf_map_delete_elem(&enter_times, &tid);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
