package io.github.louissantucci.client.command;

import io.github.louissantucci.client.grpc.FileWatcherGrpcClient;
import io.github.louissantucci.grpc.Status;
import java.util.concurrent.Callable;
import lombok.extern.slf4j.Slf4j;
import picocli.CommandLine;

@Slf4j
@CommandLine.Command(
    name = "get-watcher",
    aliases = {"get", "get-watcher"},
    description = "Gets a single file watcher by ID",
    mixinStandardHelpOptions = true)
public class GetSingleWatcherCommand implements Callable<Integer> {

  @CommandLine.Parameters(index = "0", description = "The ID of the file watcher to retrieve")
  private int watcherId;

  @Override
  public Integer call() throws Exception {
    log.info("get-watcher");
    log.info("watcherId: {}", watcherId);
    try (var grpcClient = new FileWatcherGrpcClient("localhost", 8080)) {
      var response = grpcClient.getFileWatcher(watcherId);
      if (response.getStatus() == Status.SUCCESS) {
        log.info("Watcher with ID {} retrieved successfully.", watcherId);
        log.info("Watcher details: {}", response.getFileWatcher());
      } else {
        log.error(
            "Failed to retrieve watcher with ID {}: {}", watcherId, response.getErrorMessage());
        return 1;
      }
    } catch (Exception e) {
      log.error("Error getting watcher: {}", e.getMessage());
      return 1;
    }
    return 0;
  }
}
