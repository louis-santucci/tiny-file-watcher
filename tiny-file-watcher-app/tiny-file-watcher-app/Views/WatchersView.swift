//
//  WatchersView.swift
//  tiny-file-watcher-app
//

import SwiftUI

struct WatchersView: View {
    private let watchers = SampleData.watchers

    var body: some View {
        NavigationStack {
            List(watchers) { watcher in
                NavigationLink(value: watcher) {
                    WatcherRowView(watcher: watcher)
                }
            }
            .navigationTitle("Watchers")
            .navigationDestination(for: Watcher.self) { watcher in
                WatcherDetailView(watcher: watcher)
            }
        }
    }
}

#Preview {
    WatchersView()
}
