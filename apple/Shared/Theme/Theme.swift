import SwiftUI
import CoreText

/// The Pulse palette: near-black surfaces, with green only as accents,
/// highlights and soft glows.
enum Pulse {
    // Surfaces
    static let background = Color(hex: 0x050505)
    static let backgroundDeep = Color(hex: 0x030303)
    static let panel = Color(hex: 0x0D0E0D).opacity(0.92)
    static let panelSolid = Color(hex: 0x0D0E0D)
    static let elevated = Color(hex: 0x0E0F0E)
    static let fill = Color.white.opacity(0.06)
    static let fillStrong = Color.white.opacity(0.09)
    static let hairline = Color(red: 160 / 255, green: 1, blue: 200 / 255).opacity(0.08)
    static let hairlineStrong = Color(red: 160 / 255, green: 1, blue: 200 / 255).opacity(0.16)

    // Text
    static let text = Color(hex: 0xEDF2EF)
    static let textSecondary = Color(hex: 0xA3ACA7)
    static let textTertiary = Color(hex: 0x8B938F)
    static let textDim = Color(hex: 0x7C8580)
    static let textPast = Color(hex: 0x5E6662)

    // Greens
    static let accent = Color(hex: 0x5EE6A0)
    static let accentDeep = Color(hex: 0x1E8F62)
    static let onAccent = Color(hex: 0x052016)
    static let jade = Color(hex: 0x0F766E)
    static let mintPale = Color(hex: 0xB8F5CF)
    static let emerald = Color(hex: 0x34D399)
    static let moss = Color(hex: 0x4D6B2F)
    static let sage = Color(hex: 0xC6D9A0)
    static let teal = Color(hex: 0x5EEAD4)
    static let lime = Color(hex: 0xA3E635)
    static let forest = Color(hex: 0x1F7A55)

    static var accentPill: Color { accent.opacity(0.13) }
    static var accentGlow: Color { accent.opacity(0.22) }

    // Window chrome (macOS traffic lights are drawn by the system; these are for previews only)
    static let radiusS: CGFloat = 10
    static let radiusM: CGFloat = 14
    static let radiusL: CGFloat = 20
}

// MARK: - Type

enum PulseFont {
    static let displayFamily = "Bricolage Grotesque"

    /// Registers the bundled Bricolage Grotesque variable font once per launch.
    static func register() {
        guard let url = Bundle.main.url(forResource: "BricolageGrotesque", withExtension: "ttf") else { return }
        CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
    }

    /// The variable font's base descriptor, loaded once from the bundle.
    private static let baseDescriptor: CTFontDescriptor? = {
        guard let url = Bundle.main.url(forResource: "BricolageGrotesque", withExtension: "ttf"),
              let descs = CTFontManagerCreateFontDescriptorsFromURL(url as CFURL) as? [CTFontDescriptor]
        else { return nil }
        return descs.first
    }()

    /// Bold display type (Poster's contribution): titles, headings, lyrics.
    /// Sets the variable font's weight and optical-size axes directly.
    static func display(_ size: CGFloat, weight: Font.Weight = .heavy) -> Font {
        guard let base = baseDescriptor else { return .system(size: size, weight: weight) }
        let wght: Double = switch weight {
        case .black, .heavy: 800
        case .bold: 700
        case .semibold: 600
        case .medium: 500
        default: 400
        }
        let variation: [NSNumber: NSNumber] = [
            NSNumber(value: 0x7767_6874): NSNumber(value: wght),                       // 'wght'
            NSNumber(value: 0x6F70_737A): NSNumber(value: min(96, max(12, Double(size)))), // 'opsz'
        ]
        let desc = CTFontDescriptorCreateCopyWithAttributes(
            base, [kCTFontVariationAttribute: variation] as CFDictionary)
        return Font(CTFontCreateWithFontDescriptor(desc, size, nil))
    }
}

extension Font {
    static func pulseBody(_ size: CGFloat = 14, weight: Font.Weight = .regular) -> Font {
        .system(size: size, weight: weight)
    }
    static let pulseCaption = Font.system(size: 11, weight: .semibold)
}

extension View {
    /// Tracking used by small all-caps section labels ("LIBRARY", "PLAYLISTS").
    func sectionLabel() -> some View {
        self.font(.pulseCaption)
            .tracking(1.1)
            .foregroundStyle(Pulse.textTertiary)
    }

    /// Tight negative tracking for display type.
    func displayTracking(_ size: CGFloat) -> some View {
        self.tracking(-size * 0.04)
    }
}

// MARK: - Helpers

extension Color {
    init(hex: UInt32, opacity: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: opacity
        )
    }
}
