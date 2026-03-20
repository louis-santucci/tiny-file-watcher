//
//  WatcherDetailView.swift
//  tiny-file-watcher-app
//

import SwiftUI

struct WatcherDetailView: View {
    let watcher: Watcher

    private var redirection: FileRedirection? { SampleData.redirections[watcher.name] }
    private var pendingFiles: [WatchedFile]   { SampleData.pendingFiles[watcher.name] ?? [] }

    var body: some View {
        List {
            redirectionSection
            pendingFilesSection
        }
        .navigationTitle(watcher.name)
        .navigationBarTitleDisplayMode(.large)
    }

    // MARK: - Redirection

    @ViewBuilder
    private var redirectionSection: some View {
        Section("Redirection") {
            if let r = redirection {
                LabeledContent("Target path") {
                    Text(r.targetPath)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.trailing)
                }
                LabeledContent("Auto flush") {
                    Image(systemName: r.autoFlush ? "checkmark.circle.fill" : "xmark.circle")
                        .foregroundStyle(r.autoFlush ? .green : .secondary)
                }
            } else {
                Label("No redirection configured", systemImage: "arrow.triangle.turn.up.right.circle")
                    .foregroundStyle(.secondary)
            }
        }
    }

    // MARK: - Pending Files

    @ViewBuilder
    private var pendingFilesSection: some View {
        Section {
            if pendingFiles.isEmpty {
                Label("No pending files", systemImage: "tray")
                    .foregroundStyle(.secondary)
            } else {
                ForEach(pendingFiles) { file in
                    PendingFileRowView(file: file)
                }
            }
        } header: {
            HStack {
                Text("Pending Files")
                if !pendingFiles.isEmpty {
                    Text("\(pendingFiles.count)")
                        .font(.caption2.bold())
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(.tint, in: Capsule())
                        .foregroundStyle(.white)
                }
            }
        }
    }
}

// MARK: - Pending File Row

private struct PendingFileRowView: View {
    let file: WatchedFile

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(file.fileName)
                .font(.body)
            Text(file.filePath)
                .font(.caption)
                .foregroundStyle(.secondary)
                .lineLimit(1)
                .truncationMode(.middle)
            Text(file.detectedAt, style: .relative)
                .font(.caption2)
                .foregroundStyle(.tertiary)
        }
        .padding(.vertical, 2)
    }
}

// MARK: - Preview

#Preview {
    NavigationStack {
        WatcherDetailView(watcher: SampleData.watchers[0])
    }
}
