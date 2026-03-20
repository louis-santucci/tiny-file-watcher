//
//  SampleData.swift
//  tiny-file-watcher-app
//

import Foundation

enum SampleData {
    static let watchers: [Watcher] = [
        Watcher(id: 1, name: "Downloads Monitor", sourcePath: "~/Downloads",           enabled: true,  createdAt: .now, updatedAt: .now),
        Watcher(id: 2, name: "Screenshots",       sourcePath: "~/Desktop/Screenshots", enabled: true,  createdAt: .now, updatedAt: .now),
        Watcher(id: 3, name: "Archive Inbox",     sourcePath: "/Volumes/NAS/inbox",    enabled: false, createdAt: .now, updatedAt: .now),
    ]

    static let redirections: [String: FileRedirection] = Dictionary(uniqueKeysWithValues: [
        FileRedirection(watcherName: "Downloads Monitor", targetPath: "~/Documents/Sorted",    autoFlush: true,  createdAt: .now, updatedAt: .now),
        FileRedirection(watcherName: "Screenshots",       targetPath: "~/Pictures/Screenshots", autoFlush: false, createdAt: .now, updatedAt: .now),
    ].map { ($0.watcherName, $0) })

    static let pendingFiles: [String: [WatchedFile]] = [
        "Downloads Monitor": [
            WatchedFile(id: 1, watcherId: "1", filePath: "~/Downloads/invoice_march.pdf",   fileName: "invoice_march.pdf",   flushed: false, detectedAt: Date(timeIntervalSinceNow: -3600)),
            WatchedFile(id: 2, watcherId: "1", filePath: "~/Downloads/photo_holiday.png",   fileName: "photo_holiday.png",   flushed: false, detectedAt: Date(timeIntervalSinceNow: -7200)),
            WatchedFile(id: 3, watcherId: "1", filePath: "~/Downloads/archive_backup.zip",  fileName: "archive_backup.zip",  flushed: false, detectedAt: Date(timeIntervalSinceNow: -900)),
        ],
        "Screenshots": [
            WatchedFile(id: 4, watcherId: "2", filePath: "~/Desktop/Screenshots/screen_01.png", fileName: "screen_01.png", flushed: false, detectedAt: Date(timeIntervalSinceNow: -120)),
        ],
        "Archive Inbox": [],
    ]
}
