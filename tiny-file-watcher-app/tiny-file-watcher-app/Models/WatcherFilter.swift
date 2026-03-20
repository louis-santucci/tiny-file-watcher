//
//  WatcherFilter.swift
//  tiny-file-watcher-app
//

import Foundation

enum RuleType: String, Codable, Hashable, Sendable, CaseIterable {
    case include
    case exclude
}

enum PatternType: String, Codable, Hashable, Sendable, CaseIterable {
    /// Matches by file extension (e.g. "pdf", "png").
    case `extension` = "extension"
    /// Matches by exact file name.
    case name        = "name"
    /// Matches using a glob pattern.
    case glob        = "glob"
}

struct WatcherFilter: Identifiable, Codable, Hashable, Sendable {
    let id: Int64
    var watcherName: String
    var ruleType: RuleType
    var patternType: PatternType
    var pattern: String

    enum CodingKeys: String, CodingKey {
        case id
        case watcherName = "watcher_name"
        case ruleType    = "rule_type"
        case patternType = "pattern_type"
        case pattern
    }
}
