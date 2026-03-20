//
//  Watcher.swift
//  tiny-file-watcher-app
//

import Foundation

struct Watcher: Identifiable, Codable, Hashable, Sendable {
    let id: Int64
    var name: String
    var sourcePath: String
    var enabled: Bool
    let createdAt: Date
    var updatedAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case name
        case sourcePath   = "source_path"
        case enabled
        case createdAt    = "created_at"
        case updatedAt    = "updated_at"
    }
}
