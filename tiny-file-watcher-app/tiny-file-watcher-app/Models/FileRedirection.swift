//
//  FileRedirection.swift
//  tiny-file-watcher-app
//

import Foundation

struct FileRedirection: Identifiable, Codable, Hashable, Sendable {
    /// Uses `watcherName` as the stable identity since redirections are 1-to-1 with watchers.
    var id: String { watcherName }

    var watcherName: String
    var targetPath: String
    var autoFlush: Bool
    let createdAt: Date
    var updatedAt: Date

    enum CodingKeys: String, CodingKey {
        case watcherName = "watcher_name"
        case targetPath  = "target_path"
        case autoFlush   = "auto_flush"
        case createdAt   = "created_at"
        case updatedAt   = "updated_at"
    }
}
