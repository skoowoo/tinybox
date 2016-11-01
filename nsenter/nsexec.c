#define _GNU_SOURCE
#include <stdlib.h>
#include <unistd.h>
#include <stdio.h>
#include <errno.h>
#include <string.h>

#include <linux/limits.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <sys/ioctl.h>
#include <fcntl.h>
#include <signal.h>
#include <setjmp.h>
#include <sched.h>
#include <signal.h>

/* All arguments should be above stack, because it grows down */
struct clone_arg {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char stack[4096] __attribute__ ((aligned(16)));
	char stack_ptr[0];
	jmp_buf *env;
};

#define pr_perror(fmt, ...) fprintf(stderr, "nsenter: " fmt ": %m\n", ##__VA_ARGS__)

static int child_func(void *_arg)
{
	struct clone_arg *arg = (struct clone_arg *)_arg;
	longjmp(*arg->env, 1);
}

// Use raw setns syscall for versions of glibc that don't include it (namely glibc-2.12)
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#define _GNU_SOURCE
#include "syscall.h"
#if defined(__NR_setns) && !defined(SYS_setns)
#define SYS_setns __NR_setns
#endif
#ifndef SYS_setns
#define SYS_setns 134 
#define dup3(a,b,c) dup2(a,b)
#endif
int setns(int fd, int nstype)
{
	return syscall(SYS_setns, fd, nstype);
}
#endif

static int clone_parent(jmp_buf * env) __attribute__ ((noinline));
static int clone_parent(jmp_buf * env)
{
	struct clone_arg ca;
	int child;

	ca.env = env;
	child = clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD, &ca);

	return child;
}

void nsexec()
{
	int i, tfd, self_tfd, child, consolefd = -1;
	char *namespaces[] = { "ipc", "uts", "pid", "mnt" };
	char buf[PATH_MAX], *val;
	pid_t pid;
	jmp_buf env;
	const int num = sizeof(namespaces) / sizeof(char *);

	if ((val = getenv("__TINYJAIL_INIT_PID__")) == NULL) {
		return;
    }

	pid = atoi(val);
	snprintf(buf, sizeof(buf), "%d", pid);
	if (strcmp(val, buf)) {
		pr_perror("Unable to parse _LIBCONTAINER_INITPID");
		exit(1);
	}

	/* Check that the specified process exists */
	snprintf(buf, PATH_MAX - 1, "/proc/%d/ns", pid);
	tfd = open(buf, O_DIRECTORY | O_RDONLY);
	if (tfd == -1) {
		pr_perror("Failed to open \"%s\"", buf);
		exit(1);
	}

	self_tfd = open("/proc/self/ns", O_DIRECTORY | O_RDONLY);
	if (self_tfd == -1) {
		pr_perror("Failed to open /proc/self/ns");
		exit(1);
	}

	for (i = 0; i < num; i++) {
		struct stat st;
		struct stat self_st;
		int fd;

		/* Symlinks on all namespaces exist for dead processes, but they can't be opened */
		if (fstatat(tfd, namespaces[i], &st, 0) == -1) {
			// Ignore nonexistent namespaces.
			if (errno == ENOENT)
				continue;
		}

		/* Skip namespaces we're already part of */
		if (fstatat(self_tfd, namespaces[i], &self_st, 0) != -1 &&
		    st.st_ino == self_st.st_ino) {
			continue;
		}

		fd = openat(tfd, namespaces[i], O_RDONLY);
		if (fd == -1) {
			pr_perror("Failed to open ns file %s for ns %s", buf,
				  namespaces[i]);
			exit(1);
		}
		// Set the namespace.
		if (setns(fd, 0) == -1) {
			pr_perror("Failed to setns for %s", namespaces[i]);
			exit(1);
		}
		close(fd);
	}

	close(self_tfd);
	close(tfd);

	if (setjmp(env) == 1) {
		// Child

		if (setsid() == -1) {
			pr_perror("setsid failed");
			exit(1);
		}
		if (consolefd != -1) {
			if (ioctl(consolefd, TIOCSCTTY, 0) == -1) {
				pr_perror("ioctl TIOCSCTTY failed");
				exit(1);
			}
			if (dup3(consolefd, STDIN_FILENO, 0) != STDIN_FILENO) {
				pr_perror("Failed to dup 0");
				exit(1);
			}
			if (dup3(consolefd, STDOUT_FILENO, 0) != STDOUT_FILENO) {
				pr_perror("Failed to dup 1");
				exit(1);
			}
			if (dup3(consolefd, STDERR_FILENO, 0) != STDERR_FILENO) {
				pr_perror("Failed to dup 2");
				exit(1);
			}
		}
		// Finish executing, let the Go runtime take over.
		return;
	}
	// Parent

	// We must fork to actually enter the PID namespace, use CLONE_PARENT
	// so the child can have the right parent, and we don't need to forward
	// the child's exit code or resend its death signal.
	child = clone_parent(&env);
	if (child < 0) {
		pr_perror("Unable to fork");
		exit(1);
	}

	exit(child);
}
