//
//  WatcherRowView.swift
//  tiny-file-watcher-app
//

import SwiftUI

struct WatcherRowView: View {
    let watcher: Watcher

    var body: some View {
        HStack(spacing: 12) {
            Circle()
                .fill(watcher.enabled ? Color.green : Color.secondary.opacity(0.4))
                .frame(width: 10, height: 10)

            VStack(alignment: .leading, spacing: 2) {
                Text(watcher.name)
                    .font(.body)
                    .fontWeight(.medium)

                Text(watcher.sourcePath)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }
        .padding(.vertical, 4)
    }
}

#Preview {
    List {
        WatcherRowView(watcher: Watcher(
            id: 1,
            name: "Downloads Monitor",
            sourcePath: "~/Downloads",
            enabled: true,
            createdAt: .now,
            updatedAt: .now
        ))
        WatcherRowView(watcher: Watcher(
            id: 2,
            name: "Archive Inbox",
            sourcePath: "/Volumes/NAS/inbox",
            enabled: false,
            createdAt: .now,
            updatedAt: .now
        ))
    }
}
