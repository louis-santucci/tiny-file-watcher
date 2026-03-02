package io.github.louissantucci.client.command;

import io.github.louissantucci.client.grpc.FileWatcherGrpcClient;
import java.util.concurrent.Callable;

import io.grpc.NameResolverRegistry;
import io.grpc.internal.DnsNameResolverProvider;
import lombok.extern.slf4j.Slf4j;
import picocli.CommandLine;

@Slf4j
@CommandLine.Command(
    name = "list-watchers",
    aliases = {"lw", "list-watchers", "list"},
    description = "Lists all file watchers",
    mixinStandardHelpOptions = true)
public class ListWatchersCommand implements Callable<Integer> {

  @CommandLine.Option(
      names = {"--active", "-a"},
      description = "List only active watchers")
  private Boolean active = null;

  @Override
  public Integer call() throws Exception {
    log.info("list-watchers");
    log.info("active: {}", active);
    try (var grpcClient = new FileWatcherGrpcClient("127.0.0.1", 8080)) {
      var response = grpcClient.getAllFileWatchers(active);
      var watchers = response.getFileWatchersList();
      if (watchers.isEmpty()) {
        log.info("No watchers found.");
      } else {
        log.info("Watchers:");
        watchers.forEach(watcher -> log.info("- {}", watcher));
      }
    } catch (Exception e) {
      log.error("Error listing watchers: {}", e.getMessage());
      return 1;
    }
    return 0;
  }
}
