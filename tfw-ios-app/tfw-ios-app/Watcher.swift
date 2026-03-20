//
//  Watcher.swift
//  tfw-ios-app
//
//  Created by Louis SANTUCCI on 20/03/2026.
//

import Foundation

struct Watcher: Identifiable {
    let id: Int64
    var name: String
    var sourcePath: String
    var enabled: Bool
    var createdAt: Date
    var updatedAt: Date
    
    init(id: Int64, name: String, sourcePath: String, enabled: Bool, createdAt: Date, updatedAt: Date) {
        self.id = id
        self.name = name
        self.sourcePath = sourcePath
        self.enabled = enabled
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }
}
