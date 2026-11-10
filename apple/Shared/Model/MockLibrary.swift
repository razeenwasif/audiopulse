import Foundation

/// Sample library that drives the UI until the Spotify backend is connected.
/// Artists and songs are fictional.
enum MockLibrary {
    static let lowTideArt = Artwork(colors: [0x5EE6A0, 0x0F766E, 0x052E2B], highlight: 0xE4FBB0)

    static let lowTide = Track(id: "low-tide", title: "Low Tide", artist: "Marlow Vance", album: "Salt Year",
                               genre: "Indie", duration: 256, artwork: lowTideArt)

    static let tracks: [Track] = [
        Track(id: "ferry-lights", title: "Ferry Lights", artist: "Marlow Vance", album: "Salt Year", genre: "Indie folk",
              duration: 232, artwork: Artwork(colors: [0x064E3B, 0x6EE7B7], angle: 135),
              why: "From Night Drive, same artist as Low Tide"),
        lowTide,
        Track(id: "paper-moons", title: "Paper Moons", artist: "The Lanterns", album: "Lantern Hours", genre: "Indie",
              duration: 302, artwork: Artwork(colors: [0x064E3B, 0x10B981, 0xA7F3D0], angle: 200),
              why: "In your Liked Songs, soft vocals"),
        Track(id: "slow-weather", title: "Slow Weather", artist: "Haru & Oak", album: "Small Rooms", genre: "Lo-fi",
              duration: 371, artwork: Artwork(colors: [0xD9E8C4, 0x6B8E4E, 0x1F2E17]),
              why: "Low energy, rain in the title track"),
        Track(id: "harbour-song", title: "Harbour Song", artist: "Odessa Rue", album: "Glasshouse", genre: "Indie",
              duration: 224, artwork: Artwork(colors: [0x84CC16, 0x14532D], highlight: 0xECFCCB, angle: 150),
              why: "You played it 9 times this month"),
        Track(id: "undertow", title: "Undertow", artist: "Kaiya", album: "Pressure", genre: "Dream pop",
              duration: 280, artwork: Artwork(colors: [0x042F2E, 0x14B8A6, 0xCCFBF1], angle: 210),
              why: "Quiet, reverb-heavy, from Deep Focus"),
        Track(id: "grey-morning", title: "Grey Morning", artist: "The Lanterns", album: "Lantern Hours", genre: "Indie folk",
              duration: 198, artwork: Artwork(colors: [0x1C2B1F, 0x86A873], angle: 135),
              why: "Acoustic, liked last spring"),
        Track(id: "window-seat", title: "Window Seat", artist: "Junie K", album: "Commuter", genre: "Lo-fi",
              duration: 177, artwork: Artwork(colors: [0x365314, 0xA3E635], angle: 135),
              why: "Mellow beat, no loud drops"),
        Track(id: "fernlight", title: "Fernlight", artist: "VELA", album: "Static Bloom", genre: "Electronic",
              duration: 245, artwork: Artwork(colors: [0x052E16, 0x166534], highlight: 0xBEF264, angle: 140)),
        Track(id: "glasshouse", title: "Glasshouse", artist: "Odessa Rue", album: "Glasshouse", genre: "Soul",
              duration: 211, artwork: Artwork(colors: [0x84CC16, 0x14532D], highlight: 0xECFCCB, angle: 150)),
    ]

    static func track(_ id: String) -> Track { tracks.first { $0.id == id }! }

    static let nightDrive = Playlist(
        id: "night-drive", name: "Night Drive", initials: "ND", badgeColor: 0x0F766E, badgeOnDark: true,
        artwork: Artwork(colors: [0x042F2E, 0x0F766E], angle: 135),
        tracks: ["ferry-lights", "undertow", "low-tide", "glasshouse", "paper-moons", "fernlight", "window-seat"].map(track))

    static let rainyIndie = Playlist(
        id: "rainy-indie", name: "Rainy Indie", initials: "RI", badgeColor: 0x5EEAD4, badgeOnDark: false,
        artwork: Artwork(colors: [0x5EEAD4, 0x134E4A, 0x052E2B], highlight: 0xCCFBF1, angle: 170),
        tracks: ["paper-moons", "ferry-lights", "slow-weather", "harbour-song", "undertow", "grey-morning", "window-seat"].map(track),
        updated: true, prompt: "rainy day indie, nothing too loud")

    static let playlists: [Playlist] = [
        nightDrive,
        rainyIndie,
        Playlist(id: "deep-focus", name: "Deep Focus", initials: "DF", badgeColor: 0xB8F5CF, badgeOnDark: false,
                 artwork: Artwork(colors: [0x14532D, 0x86EFAC], angle: 135),
                 tracks: ["undertow", "slow-weather", "fernlight"].map(track), updated: true),
        Playlist(id: "sunday-kitchen", name: "Sunday Kitchen", initials: "SK", badgeColor: 0x4D6B2F, badgeOnDark: true,
                 artwork: Artwork(colors: [0x365314, 0xC6D9A0], angle: 135),
                 tracks: ["glasshouse", "harbour-song", "grey-morning"].map(track)),
        Playlist(id: "late-jazz", name: "Late Jazz", initials: "LJ", badgeColor: 0xC6D9A0, badgeOnDark: false,
                 artwork: Artwork(colors: [0x1C2B1F, 0x86A873], angle: 135),
                 tracks: ["slow-weather", "grey-morning"].map(track)),
    ]

    static let picks: [Track] = ["glasshouse", "paper-moons", "undertow", "slow-weather", "fernlight"].map(track)

    static let builtFromLibrary: [(name: String, why: String, art: Artwork)] = [
        ("Rainy Commute", "Your mellow likes, 25 songs", Artwork(colors: [0x134E4A, 0x5EEAD4], angle: 135)),
        ("Throwback Jams", "From “90s to 2000s classics”", Artwork(colors: [0x365314, 0xA3E635], angle: 135)),
        ("Late Jazz", "Sorted from Liked Songs", Artwork(colors: [0x1C2B1F, 0x86A873], angle: 135)),
    ]

    static let genres: [Genre] = [
        Genre(name: "Indie", count: 312, color: 0x5EE6A0),
        Genre(name: "Electronic", count: 248, color: 0x2DD4BF),
        Genre(name: "Hip-Hop", count: 190, color: 0xA3E635),
        Genre(name: "Jazz", count: 84, color: 0xC6D9A0),
        Genre(name: "Soul", count: 77, color: 0x86A873),
    ]

    static let likedCount = 1284

    static let prompts = ["Smart shuffle Night Drive", "Make a playlist from my 2000s likes", "What have I been into lately?"]

    static let lowTideLyrics: [LyricLine] = [
        LyricLine(time: 0, text: "♪"),
        LyricLine(time: 58, text: "salt on the window, salt in my coat"),
        LyricLine(time: 91, text: "the harbour lights go out one by one"),
        LyricLine(time: 104, text: "and I keep the radio low so I can hear you come"),
        LyricLine(time: 112, text: "back across the water"),
        LyricLine(time: 121, text: "back across the water"),
        LyricLine(time: 130, text: "salt in the engine, salt in the year"),
        LyricLine(time: 144, text: "every tide that leaves me brings you near"),
        LyricLine(time: 170, text: "♪"),
    ]

    static func lyrics(for track: Track) -> [LyricLine] {
        track.id == lowTide.id ? lowTideLyrics : [LyricLine(time: 0, text: "Lyrics will appear here once synced from LRCLIB.")]
    }
}
