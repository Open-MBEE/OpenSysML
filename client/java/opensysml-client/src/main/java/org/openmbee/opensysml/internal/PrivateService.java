package org.openmbee.opensysml.internal;

import org.openmbee.opensysml.ServiceStartException;
import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.List;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.TimeUnit;

/**
 * A {@code sysml-grpc} started as a child of this JVM, on a port the kernel chose, which no other
 * process is told about.
 *
 * <p>No orphan is possible and no exit hook is involved: the child is started with
 * {@code -exit-with-parent}, so it reads its own stdin and shuts down at end of file, and this
 * class holds the write end of that pipe and never writes to it. The kernel closes the pipe when
 * this JVM dies — including {@code SIGKILL}, a fatal JVM error, and {@code Runtime.halt} — so the
 * child sees end of file and exits. Windows has the same guarantee for the anonymous pipe
 * {@code ProcessBuilder} creates.
 */
public final class PrivateService {

  private static final int LOG_LINES = 50;

  private final Process process;
  private final Path binary;
  private final OutputStream childStdin;
  private final Deque<String> log = new ArrayDeque<>();

  /** Owning connections, guarded by {@link ServiceRegistry}'s lock. */
  private int references;

  /** Set once, by the thread that started the child, before any caller sees the service. */
  private volatile String address = "";

  private PrivateService(Process process, Path binary) {
    this.process = process;
    this.binary = binary;
    this.childStdin = process.getOutputStream();
  }

  /**
   * Starts a private service and waits for the address it reports.
   *
   * @param binary the service binary
   * @param startupTimeout how long it has to report an address
   * @return the running service, with no references taken yet
   * @throws ServiceStartException if it could not be started, or reported no address in time
   */
  static PrivateService start(Path binary, Duration startupTimeout) {
    ProcessBuilder builder =
        new ProcessBuilder(
            binary.toString(),
            "-port",
            "0",
            "-health-port",
            "0",
            "-report-address",
            "-exit-with-parent");
    Process process;
    try {
      process = builder.start();
    } catch (IOException e) {
      throw new ServiceStartException("sysml-grpc at " + binary + " could not be started", e);
    }

    PrivateService service = new PrivateService(process, binary);
    BlockingQueue<String> reported = new ArrayBlockingQueue<>(1);
    service.pump(process.getInputStream(), "opensysml-service-stdout", reported);
    service.pump(process.getErrorStream(), "opensysml-service-stderr", null);

    String address;
    try {
      address = reported.poll(startupTimeout.toMillis(), TimeUnit.MILLISECONDS);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      service.stop();
      throw new ServiceStartException(
          "waiting for sysml-grpc to report its address was interrupted", e);
    }
    if (address == null || address.isBlank()) {
      String why =
          process.isAlive()
              ? "did not report a listening address within " + startupTimeout
              : "exited with status " + process.exitValue() + " while starting";
      service.stop();
      throw new ServiceStartException(
          "sysml-grpc at " + binary + " " + why + service.reportedLog());
    }
    service.address = address.trim();
    return service;
  }

  /**
   * The {@code host:port} the child reported.
   *
   * @return the address to dial
   */
  public String address() {
    return address;
  }

  /**
   * The binary this service runs.
   *
   * @return the binary path
   */
  public Path binary() {
    return binary;
  }

  /**
   * Whether the child is still running.
   *
   * @return {@code true} while it lives
   */
  public boolean isAlive() {
    return process.isAlive();
  }

  /**
   * The child's process id, for a test or a log line.
   *
   * @return the process id
   */
  public long pid() {
    return process.pid();
  }

  int references() {
    return references;
  }

  int retain() {
    return ++references;
  }

  int release() {
    return --references;
  }

  /**
   * Stops the child: closes its stdin, which is what it shuts down on, and only escalates if it
   * does not go.
   */
  void stop() {
    try {
      childStdin.close();
    } catch (IOException e) {
      // A pipe that is already gone needs no closing.
    }
    try {
      if (!process.waitFor(5, TimeUnit.SECONDS)) {
        process.destroy();
        if (!process.waitFor(2, TimeUnit.SECONDS)) {
          process.destroyForcibly();
        }
      }
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      process.destroyForcibly();
    }
  }

  /** Drains a stream on a daemon thread, keeping the last lines for an error message. */
  private void pump(InputStream stream, String threadName, BlockingQueue<String> firstLine) {
    Thread thread =
        new Thread(
            () -> {
              boolean reported = false;
              try (BufferedReader reader =
                  new BufferedReader(new InputStreamReader(stream, StandardCharsets.UTF_8))) {
                String line;
                while ((line = reader.readLine()) != null) {
                  recordLogLine(line);
                  if (!reported && firstLine != null) {
                    reported = firstLine.offer(line);
                  }
                }
              } catch (IOException e) {
                // The child's exit closes its streams; that is the end of the log, not a failure.
              } finally {
                // End of stream without a line: unblock the caller so it reports the exit.
                if (!reported && firstLine != null && !firstLine.offer("")) {
                  recordLogLine("could not report the end of " + threadName);
                }
              }
            },
            threadName);
    thread.setDaemon(true);
    thread.start();
  }

  private void recordLogLine(String line) {
    synchronized (log) {
      if (log.size() == LOG_LINES) {
        log.removeFirst();
      }
      log.addLast(line);
    }
  }

  private String reportedLog() {
    List<String> lines;
    synchronized (log) {
      lines = new ArrayList<>(log);
    }
    return lines.isEmpty() ? "" : "; it logged: " + String.join(" | ", lines);
  }
}
