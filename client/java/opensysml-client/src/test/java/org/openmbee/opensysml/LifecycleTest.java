package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.internal.ServiceRegistry;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.nio.file.attribute.FileTime;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** Who owns the service, when it stops, and that nothing races or leaks. */
class LifecycleTest {

  @BeforeEach
  @AfterEach
  void noServiceIsLeftRunning() {
    ServiceRegistry.stopAll();
  }

  @Test
  void connectionsShareOnePrivateServiceAndItsParseCache() {
    try (Connection first = Connection.open(ServiceBinary.options().build());
        Connection second = Connection.open(ServiceBinary.options().build())) {
      assertEquals(1, ServiceRegistry.sharedServiceCount());
      assertEquals(first.address(), second.address());
      assertTrue(first.ownsService());
      String hash = first.parse("package Demo { part def Vehicle; }").hash();
      assertEquals(hash, second.parse("package Demo { part def Vehicle; }").hash());
      assertEquals("Vehicle", second.model(hash).symbol("Demo::Vehicle").name());
    }
    assertEquals(0, ServiceRegistry.sharedServiceCount());
  }

  @Test
  void theLastConnectionToCloseStopsTheChild() throws Exception {
    long pid;
    Connection first = Connection.open(ServiceBinary.options().build());
    Connection second = Connection.open(ServiceBinary.options().build());
    try {
      pid = servicePid(first);
      first.close();
      assertTrue(ProcessHandle.of(pid).orElseThrow().isAlive(), "still held by the second");
    } finally {
      second.close();
    }
    assertTrue(waitForExit(pid), "the child outlived its last connection");
  }

  @Test
  void replacingTheBinaryDoesNotShareTheServiceRunningWhatItDisplaced(@TempDir Path directory)
      throws Exception {
    // A download replaces the cached binary in place, so sharing by path alone would hand a
    // connection asking for the new release the child running the old one.
    Path binary = directory.resolve("sysml-grpc");
    Files.copy(ServiceBinary.required(), binary);
    binary.toFile().setExecutable(true, true);

    try (Connection first = Connection.open(ConnectionOptions.builder().binaryPath(binary).build())) {
      Path replacement = directory.resolve("sysml-grpc.new");
      Files.copy(ServiceBinary.required(), replacement);
      replacement.toFile().setExecutable(true, true);
      Files.move(replacement, binary, StandardCopyOption.REPLACE_EXISTING);
      Files.setLastModifiedTime(binary, FileTime.from(Instant.now().plusSeconds(2)));

      try (Connection second =
          Connection.open(ConnectionOptions.builder().binaryPath(binary).build())) {
        assertEquals(2, ServiceRegistry.sharedServiceCount());
        assertNotEquals(first.address(), second.address());
      }
    }
    assertEquals(0, ServiceRegistry.sharedServiceCount());
  }

  @Test
  void oneBinaryUnderTwoNamesSharesOneService(@TempDir Path directory) throws Exception {
    // A download starts its service from a digest-named link to the cache, so keying by name
    // would start a second child for the connection that resolved the cache itself.
    Path binary = directory.resolve("sysml-grpc");
    Files.copy(ServiceBinary.required(), binary);
    binary.toFile().setExecutable(true, true);
    Path link = directory.resolve("sysml-grpc-abcdef");
    Files.createLink(link, binary);

    try (Connection cached = Connection.open(ConnectionOptions.builder().binaryPath(binary).build());
        Connection linked = Connection.open(ConnectionOptions.builder().binaryPath(link).build())) {
      assertEquals(1, ServiceRegistry.sharedServiceCount());
      assertEquals(cached.address(), linked.address());
    }
    assertEquals(0, ServiceRegistry.sharedServiceCount());
  }

  @Test
  void anIsolatedServiceIsNotShared() throws Exception {
    try (Connection shared = Connection.open(ServiceBinary.options().build());
        Connection isolated =
            Connection.open(ServiceBinary.options().isolatedService(true).build())) {
      assertEquals(1, ServiceRegistry.sharedServiceCount(), "the isolated one is not registered");
      assertNotEquals(shared.address(), isolated.address());
      long pid = servicePid(isolated);
      isolated.close();
      assertTrue(waitForExit(pid), "an isolated child outlived its only connection");
      assertTrue(ProcessHandle.of(servicePid(shared)).orElseThrow().isAlive());
    }
  }

  @Test
  void eightThreadsOpeningAtOnceStartOneChild() throws Exception {
    int threads = 8;
    CyclicBarrier start = new CyclicBarrier(threads);
    ExecutorService pool = Executors.newFixedThreadPool(threads);
    List<Connection> opened = new ArrayList<>();
    try {
      List<Callable<Connection>> tasks = new ArrayList<>();
      for (int i = 0; i < threads; i++) {
        tasks.add(
            () -> {
              start.await(30, TimeUnit.SECONDS);
              return Connection.open(ServiceBinary.options().build());
            });
      }
      Set<String> addresses = new HashSet<>();
      for (Future<Connection> future : pool.invokeAll(tasks)) {
        Connection connection = future.get(60, TimeUnit.SECONDS);
        opened.add(connection);
        addresses.add(connection.address());
      }
      assertEquals(1, addresses.size(), "the threads raced and started more than one child: " + addresses);
      assertEquals(1, ServiceRegistry.sharedServiceCount());
    } finally {
      pool.shutdownNow();
      opened.forEach(Connection::close);
    }
    assertEquals(0, ServiceRegistry.sharedServiceCount());
  }

  @Test
  void closingIsIdempotentAndReleasesTheServiceOnce() throws Exception {
    Connection first = Connection.open(ServiceBinary.options().build());
    long pid = servicePid(first);
    try (Connection second = Connection.open(ServiceBinary.options().build())) {
      first.close();
      first.close();
      first.close();
      assertTrue(ProcessHandle.of(pid).orElseThrow().isAlive(), "a repeated close stopped it early");
      assertFalse(second.parse("package Demo;").hash().isBlank());
    }
    assertTrue(waitForExit(pid));
  }

  @Test
  void anExternalServiceOutlivesTheConnectionsToIt() throws Exception {
    long pid;
    String address;
    try (Connection owner = Connection.open(ServiceBinary.options().build())) {
      pid = servicePid(owner);
      address = owner.address();
      int separator = address.lastIndexOf(':');
      ConnectionOptions external =
          ConnectionOptions.builder()
              .service(address.substring(0, separator), Integer.parseInt(address.substring(separator + 1)))
              .build();
      try (Connection guest = Connection.open(external)) {
        assertFalse(guest.ownsService());
        assertEquals(address, guest.address());
        assertFalse(guest.parse("package Demo;").hash().isBlank());
      }
      assertTrue(ProcessHandle.of(pid).orElseThrow().isAlive(), "closing a guest stopped the service");
      assertFalse(owner.parse("package Demo;").hash().isBlank());
    }
    assertTrue(waitForExit(pid));
  }

  @Test
  void anUnreachableExternalServiceIsATransportFailure() {
    ConnectionOptions options =
        ConnectionOptions.builder()
            .service("127.0.0.1", 1)
            .requestTimeout(Duration.ofSeconds(2))
            .build();
    assertThrows(TransportException.class, () -> Connection.open(options));
  }

  @Test
  void stoppingSharedServicesLeavesNoChildAndNoLiveThread() throws Exception {
    Connection connection = Connection.open(ServiceBinary.options().build());
    long pid = servicePid(connection);
    assertEquals(1, Connection.stopSharedServices());
    assertTrue(waitForExit(pid));
    assertEquals(0, ServiceRegistry.sharedServiceCount());
    assertEquals(0, Connection.stopSharedServices());
    connection.close(); // the host's connection is still closed normally
    assertTrue(nonDaemonClientThreads().isEmpty(), "left a non-daemon thread: " + nonDaemonClientThreads());
  }

  private static long servicePid(Connection connection) {
    return connection.ownedService().orElseThrow().pid();
  }

  private static boolean waitForExit(long pid) throws InterruptedException {
    ProcessHandle process = ProcessHandle.of(pid).filter(ProcessHandle::isAlive).orElse(null);
    if (process == null) {
      return true;
    }
    try {
      process.onExit().get(10, TimeUnit.SECONDS);
      return true;
    } catch (TimeoutException e) {
      return !process.isAlive();
    } catch (ExecutionException e) {
      throw new AssertionError("waiting for process " + pid, e);
    }
  }

  private static List<String> nonDaemonClientThreads() {
    return Thread.getAllStackTraces().keySet().stream()
        .filter(thread -> !thread.isDaemon())
        .filter(thread -> thread.getName().startsWith("opensysml"))
        .map(Thread::getName)
        .toList();
  }
}
