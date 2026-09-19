package org.openmbee.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.File;
import java.lang.ref.Reference;
import java.lang.ref.ReferenceQueue;
import java.lang.ref.WeakReference;
import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.junit.jupiter.api.Test;

/** A host application that loads and unloads the client repeatedly leaves nothing behind. */
class ClassLoaderTest {

  @Test
  void repeatedLoadAndUnloadLeavesNoChildAndNoUncollectableCopy() throws Exception {
    Path binary = ServiceBinary.required();
    List<Long> pids = new ArrayList<>();
    List<WeakReference<ClassLoader>> loaders = new ArrayList<>();
    ReferenceQueue<ClassLoader> collectedLoaders = new ReferenceQueue<>();

    for (int round = 0; round < 3; round++) {
      // Platform parent, so this copy of the client is a different class from the test's own.
      URLClassLoader loader =
          new URLClassLoader(
              "opensysml-host-" + round, classpath(), ClassLoader.getPlatformClassLoader());
      Method run = loader.loadClass(ClassLoaderProbe.class.getName()).getMethod("run", String.class);
      long pid = (long) run.invoke(null, binary.toString());
      pids.add(pid);
      loaders.add(new WeakReference<>(loader, collectedLoaders));
      loader.close();
    }

    // Each copy of the client owned its own child, since the registry is per classloader.
    assertEquals3DistinctChildren(pids);
    for (long pid : pids) {
      assertTrue(waitForExit(pid), "a child outlived the classloader that started it: " + pid);
    }
    assertTrue(
        nonDaemonThreads().isEmpty(), "an unloaded copy left a non-daemon thread: " + nonDaemonThreads());
    assertTrue(
        collected(loaders, collectedLoaders), "an unloaded copy of the client could not be collected");
  }

  private static void assertEquals3DistinctChildren(List<Long> pids) {
    assertFalse(pids.isEmpty());
    assertEquals(
        pids.size(), pids.stream().distinct().count(), "the copies shared a child: " + pids);
  }

  private static URL[] classpath() throws Exception {
    String[] entries = System.getProperty("java.class.path").split(File.pathSeparator);
    List<URL> urls = new ArrayList<>();
    for (String entry : entries) {
      urls.add(Path.of(entry).toUri().toURL());
    }
    return urls.toArray(new URL[0]);
  }

  // Generous: on 17 the JDK's own HttpClient selector thread outlives the closed client until it
  // is collected, and until that thread exits it pins the loader that created the client.
  private static boolean collected(
      List<WeakReference<ClassLoader>> loaders, ReferenceQueue<ClassLoader> collected)
      throws InterruptedException {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
    int remaining = loaders.size();
    while (remaining > 0 && System.nanoTime() < deadline) {
      System.gc();
      Reference<? extends ClassLoader> reference = collected.remove(100);
      if (reference != null) {
        remaining--;
        while (collected.poll() != null) {
          remaining--;
        }
      }
    }
    if (remaining == 0) {
      return true;
    }
    assertNull(loaders.get(0).get());
    return false;
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

  private static List<String> nonDaemonThreads() {
    return Thread.getAllStackTraces().keySet().stream()
        .filter(thread -> !thread.isDaemon())
        .filter(thread -> thread.getName().startsWith("opensysml"))
        .map(Thread::getName)
        .toList();
  }
}
