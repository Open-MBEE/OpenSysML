package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.DisabledOnOs;
import org.junit.jupiter.api.condition.OS;

/**
 * The no-orphan guarantee: the child dies with the JVM even when the JVM had no chance to run any
 * code, so no shutdown hook, {@code ProcessHandle.onExit} or finalizer is involved.
 */
class OrphanSafetyTest {

  @Test
  @DisabledOnOs(value = OS.WINDOWS, disabledReason = "SIGKILL is a POSIX signal")
  void aSigkilledJvmLeavesNoChild() throws Exception {
    assertChildDiesWith("kill");
  }

  @Test
  void aHaltedJvmLeavesNoChild() throws Exception {
    assertChildDiesWith("halt");
  }

  /** Starts a JVM that owns a service, ends it abruptly, and asserts the service went with it. */
  private void assertChildDiesWith(String how) throws Exception {
    Path binary = ServiceBinary.required();
    ProcessBuilder builder =
        new ProcessBuilder(
            ProcessHandle.current().info().command().orElseThrow(),
            "-cp",
            System.getProperty("java.class.path"),
            OrphanSafetyChild.class.getName(),
            binary.toString(),
            how);
    builder.redirectErrorStream(false);
    Process jvm = builder.start();
    long servicePid;
    try (BufferedReader reader =
        new BufferedReader(new InputStreamReader(jvm.getInputStream(), StandardCharsets.UTF_8))) {
      String line = reader.readLine();
      assertTrue(line != null && !line.isBlank(), "the child JVM reported no service pid");
      servicePid = Long.parseLong(line.trim());
      assertTrue(ProcessHandle.of(servicePid).orElseThrow().isAlive());

      if (how.equals("kill")) {
        // destroyForcibly is SIGKILL on POSIX: the JVM runs nothing on its way out.
        jvm.destroyForcibly();
      }
      assertTrue(jvm.waitFor(30, TimeUnit.SECONDS), "the child JVM did not end");
    }
    assertTrue(waitForExit(servicePid), "the service outlived the JVM that owned it");
    assertFalse(ProcessHandle.of(servicePid).map(ProcessHandle::isAlive).orElse(false));
  }

  private static boolean waitForExit(long pid) throws InterruptedException {
    ProcessHandle process = ProcessHandle.of(pid).filter(ProcessHandle::isAlive).orElse(null);
    if (process == null) {
      return true;
    }
    try {
      process.onExit().get(20, TimeUnit.SECONDS);
      return true;
    } catch (TimeoutException e) {
      return !process.isAlive();
    } catch (ExecutionException e) {
      throw new AssertionError("waiting for process " + pid, e);
    }
  }
}
