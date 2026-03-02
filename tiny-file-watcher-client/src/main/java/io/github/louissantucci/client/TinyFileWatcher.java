package io.github.louissantucci.client;

import io.github.louissantucci.client.command.GetSingleWatcherCommand;
import io.github.louissantucci.client.command.ListWatchersCommand;
import lombok.extern.slf4j.Slf4j;
import picocli.CommandLine;

@CommandLine.Command(
    name = "ftw",
    description = "Gets information about file watchers and their status",
    mixinStandardHelpOptions = true,
    version = "ftw 1.0",
    subcommands = {
      ListWatchersCommand.class,
      GetSingleWatcherCommand.class,
    })
@Slf4j
public class TinyFileWatcher {

  static void main(String[] args) {
    int exitCode = new CommandLine(new TinyFileWatcher()).execute(args);
    System.exit(exitCode);
  }
}
