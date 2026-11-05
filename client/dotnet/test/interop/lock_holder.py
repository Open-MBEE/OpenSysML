"""Hold or try the shared cache lock exactly as the Python client takes it.

The .NET client's interoperability tests run this beside it. The open and the lock
calls are those of _locked_once and _file_locked in client/python/opensysml/binary.py:
fcntl.lockf over the whole file on POSIX, msvcrt.locking over its first byte on
Windows, on a descriptor from os.open.

Usage:
    lock_holder.py hold <lock-file>   take the lock as the client does, waiting for it;
                                      print "held", release at a line or end of input,
                                      then print "released"
    lock_holder.py try <lock-file>    take it without waiting; print "free" (and
                                      release it) or "busy"
"""

import errno
import os
import sys


def open_lock(path):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    return os.open(path, os.O_RDWR | os.O_CREAT, 0o600)


def lock(fd, wait):
    if os.name == 'nt':
        import msvcrt

        msvcrt.locking(fd, msvcrt.LK_LOCK if wait else msvcrt.LK_NBLCK, 1)
    else:
        import fcntl

        fcntl.lockf(fd, fcntl.LOCK_EX if wait else fcntl.LOCK_EX | fcntl.LOCK_NB)


def unlock(fd):
    if os.name == 'nt':
        import msvcrt

        os.lseek(fd, 0, os.SEEK_SET)
        msvcrt.locking(fd, msvcrt.LK_UNLCK, 1)
    else:
        import fcntl

        fcntl.lockf(fd, fcntl.LOCK_UN)


def say(word):
    print(word, flush=True)


def main(argv):
    if len(argv) != 3 or argv[1] not in ('hold', 'try'):
        sys.stderr.write(__doc__)
        return 2
    mode, path = argv[1], argv[2]
    fd = open_lock(path)
    try:
        if mode == 'try':
            try:
                lock(fd, wait=False)
            except OSError as e:
                # POSIX reports a held lock as EACCES or EAGAIN, msvcrt as EACCES or EDEADLOCK.
                if e.errno in (errno.EACCES, errno.EAGAIN, getattr(errno, 'EDEADLOCK', errno.EACCES)):
                    say('busy')
                    return 0
                raise
            unlock(fd)
            say('free')
            return 0
        lock(fd, wait=True)
        say('held')
        sys.stdin.readline()
        unlock(fd)
        say('released')
        return 0
    finally:
        os.close(fd)


if __name__ == '__main__':
    sys.exit(main(sys.argv))
