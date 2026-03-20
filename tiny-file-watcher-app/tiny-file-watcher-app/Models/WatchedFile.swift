//
//  WatchedFile.swift
//  tiny-file-watcher-app
//

import Foundation

struct WatchedFile: Identifiable, Codable, Hashable, Sendable {
    let id: Int64
    var watcherId: String
    var filePath: String
    var fileName: String
    var flushed: Bool
    let detectedAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case watcherId  = "watcher_id"
        case filePath   = "file_path"
        case fileName   = "file_name"
        case flushed
        case detectedAt = "detected_at"
    }
}
