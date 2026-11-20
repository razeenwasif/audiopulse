import AppKit

/// App-wide bare-key shortcuts (space = play/pause, esc = close overlays).
/// A local event monitor is used instead of making the root view focusable,
/// which would mark every button as focused. Keys typed into a text field
/// pass straight through.
@MainActor
final class WindowKeys {
    private var monitor: Any?

    func install(player: PlayerModel, router: AppRouter) {
        guard monitor == nil else { return }
        monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { event in
            let editingText = NSApp.keyWindow?.firstResponder is NSText
            let bare = event.modifierFlags.intersection([.command, .option, .control]).isEmpty
            guard bare else { return event }

            switch event.keyCode {
            case 49 where !editingText: // space
                player.togglePlay()
                return nil
            case 53: // escape
                if router.showAsk { router.closeAsk(); return nil }
                if router.showLyrics { router.toggleLyrics(); return nil }
                return event
            default:
                return event
            }
        }
    }

    func remove() {
        if let monitor { NSEvent.removeMonitor(monitor) }
        monitor = nil
    }
}
