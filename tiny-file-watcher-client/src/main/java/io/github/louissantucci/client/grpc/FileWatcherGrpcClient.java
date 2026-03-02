package io.github.louissantucci.client.grpc;

import io.github.louissantucci.grpc.*;
import io.grpc.ManagedChannel;
import io.grpc.NameResolverRegistry;
import io.grpc.internal.DnsNameResolverProvider;

public class FileWatcherGrpcClient implements AutoCloseable {

  private final ManagedChannel channel;
  private final FileWatcherCrudGrpc.FileWatcherCrudBlockingStub blockingStub;

  public FileWatcherGrpcClient(String host, int port) {
    NameResolverRegistry.getDefaultRegistry().register(new DnsNameResolverProvider());
    this.channel =
        io.grpc.ManagedChannelBuilder.forAddress(host, port).usePlaintext().build();
    this.blockingStub = FileWatcherCrudGrpc.newBlockingStub(channel);
  }

  public FileWatcherListResponse getAllFileWatchers(Boolean active) {
    GetAllFileWatchersRequest.Builder builder = GetAllFileWatchersRequest.newBuilder();
    if (active != null) {
      builder.setActive(active);
    }
    return blockingStub.getAllFileWatchers(builder.build());
  }

  public FileWatcherResponse getFileWatcher(long id) {
    GetFileWatcherRequest request = GetFileWatcherRequest.newBuilder().setId(id).build();
    return blockingStub.getFileWatcher(request);
  }

  @Override
  public void close() throws Exception {
    channel.shutdown();
  }
}
