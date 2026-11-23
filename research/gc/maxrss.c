#include <stdio.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/resource.h>
int main(int argc, char **argv) {
    pid_t p = fork();
    if (p == 0) { execvp(argv[1], argv + 1); _exit(127); }
    int st; struct rusage ru;
    wait4(p, &st, 0, &ru);
    fprintf(stderr, "MAXRSS_KB=%ld\n", ru.ru_maxrss);
    return WIFEXITED(st) ? WEXITSTATUS(st) : 1;
}
