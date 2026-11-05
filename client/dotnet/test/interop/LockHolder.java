import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;

/**
 * Holds or tries the shared cache lock exactly as the Java client takes it.
 *
 * <p>The .NET client's interoperability tests run this beside it, with {@code java LockHolder.java}.
 * The open and the lock calls are those of {@code withCacheLock}, {@code lockFile} and {@code held}
 * in client/java/opensysml-client/.../internal/BinaryDownloader.java: a {@link FileChannel} opened
 * to create and write, and {@link FileChannel#tryLock()} over the whole file, polled every 25 ms.
 *
 * <p>Usage: {@code hold <lock-file>} takes the lock as the client does, prints {@code held}, releases
 * it at a line or end of input and prints {@code released}; {@code try <lock-file>} takes it once
 * without waiting and prints {@code free} (releasing it) or {@code busy}.
 */
public final class LockHolder {
  private LockHolder() {}

  public static void main(String[] args) throws IOException, InterruptedException {
    if (args.length != 2 || !(args[0].equals("hold") || args[0].equals("try"))) {
      System.err.println("usage: java LockHolder.java hold|try <lock-file>");
      System.exit(2);
    }
    Path path = Paths.get(args[1]);
    Files.createDirectories(path.getParent());
    try (FileChannel channel =
        FileChannel.open(path, StandardOpenOption.CREATE, StandardOpenOption.WRITE)) {
      if (args[0].equals("try")) {
        FileLock lock = channel.tryLock();
        if (lock == null) {
          say("busy");
          return;
        }
        lock.release();
        say("free");
        return;
      }
      FileLock lock;
      while ((lock = channel.tryLock()) == null) {
        Thread.sleep(25);
      }
      say("held");
      new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8)).readLine();
      lock.release();
      say("released");
    }
  }

  private static void say(String word) {
    System.out.println(word);
    System.out.flush();
  }
}
