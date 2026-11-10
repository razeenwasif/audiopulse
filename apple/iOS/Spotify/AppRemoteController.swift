import Foundation
import SpotifyiOS

/// Wraps Spotify's App Remote: connects to the Spotify app on this iPhone,
/// wakes it when needed, and relays player-state changes.
@MainActor
final class AppRemoteController: NSObject {
    let appRemote: SPTAppRemote

    var onConnected: (() -> Void)?
    var onDisconnected: ((String?) -> Void)?
    var onState: ((SPTAppRemotePlayerState) -> Void)?
    /// A fresh App Remote token arrived from the Spotify app.
    var onToken: ((String) -> Void)?

    init(clientID: String) {
        let config = SPTConfiguration(clientID: clientID, redirectURL: URL(string: SpotifyConfig.redirectURI)!)
        appRemote = SPTAppRemote(configuration: config, logLevel: .error)
        super.init()
        appRemote.delegate = self
    }

    var isConnected: Bool { appRemote.isConnected }
    var player: SPTAppRemotePlayerAPI? { appRemote.playerAPI }

    /// Connects to an already-running Spotify app. Fails quietly if it isn't running.
    func connect(token: String) {
        guard !appRemote.isConnected else { return }
        appRemote.connectionParameters.accessToken = token
        appRemote.connect()
    }

    func disconnect() {
        if appRemote.isConnected { appRemote.disconnect() }
    }

    /// Opens the Spotify app (waking it if needed), starts `uri`, and returns
    /// here via audiopulse://spotify-callback with a token.
    func wake(playing uri: String) {
        appRemote.authorizeAndPlayURI(uri, asRadio: false, additionalScopes: nil) { [weak self] ok in
            if !ok { self?.onDisconnected?("The Spotify app isn’t installed on this iPhone.") }
        }
    }

    /// Handles the redirect from the Spotify app. Returns true if it was ours.
    func handle(_ url: URL) -> Bool {
        guard let params = appRemote.authorizationParameters(from: url) else { return false }
        if let token = params[SPTAppRemoteAccessTokenKey] {
            onToken?(token)
            appRemote.connectionParameters.accessToken = token
            appRemote.connect()
        } else if let message = params[SPTAppRemoteErrorDescriptionKey] {
            onDisconnected?(message)
        }
        return true
    }
}

extension AppRemoteController: SPTAppRemoteDelegate, SPTAppRemotePlayerStateDelegate {
    nonisolated func appRemoteDidEstablishConnection(_ appRemote: SPTAppRemote) {
        MainActor.assumeIsolated {
            appRemote.playerAPI?.delegate = self
            appRemote.playerAPI?.subscribe(toPlayerState: { _, _ in })
            appRemote.playerAPI?.getPlayerState { [weak self] result, _ in
                if let state = result as? SPTAppRemotePlayerState { self?.onState?(state) }
            }
            onConnected?()
        }
    }

    nonisolated func appRemote(_ appRemote: SPTAppRemote, didFailConnectionAttemptWithError error: Error?) {
        let message = error?.localizedDescription
        MainActor.assumeIsolated { onDisconnected?(message) }
    }

    nonisolated func appRemote(_ appRemote: SPTAppRemote, didDisconnectWithError error: Error?) {
        let message = error?.localizedDescription
        MainActor.assumeIsolated { onDisconnected?(message) }
    }

    nonisolated func playerStateDidChange(_ playerState: SPTAppRemotePlayerState) {
        MainActor.assumeIsolated { onState?(playerState) }
    }
}
