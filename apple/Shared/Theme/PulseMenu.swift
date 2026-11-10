import SwiftUI

/// Custom dropdown menus. A trigger reports its frame in the root coordinate
/// space; the root view (`.pulseMenuHost()`) draws the menu panel over
/// everything, so no system `Menu`, `contextMenu` or popover chrome appears.

struct PulseMenuItem: Identifiable {
    let id = UUID()
    var title: String
    var systemImage: String
    var shortcut: String? = nil
    var destructive = false
    var action: () -> Void
}

@MainActor @Observable
final class PulseMenuCenter {
    struct Presented {
        let items: [PulseMenuItem]
        let anchor: CGRect
        let alignTrailing: Bool
    }

    var presented: Presented?

    func show(_ items: [PulseMenuItem], from anchor: CGRect, alignTrailing: Bool) {
        withAnimation(.spring(response: 0.28, dampingFraction: 0.85)) {
            presented = Presented(items: items, anchor: anchor, alignTrailing: alignTrailing)
        }
    }

    func dismiss() {
        withAnimation(.easeOut(duration: 0.14)) { presented = nil }
    }
}

let pulseRootSpace = "pulseRoot"

private struct PulseMenuHost: ViewModifier {
    @State private var center = PulseMenuCenter()

    func body(content: Content) -> some View {
        content
            .environment(center)
            .coordinateSpace(name: pulseRootSpace)
            .overlay {
                GeometryReader { geo in
                    if let p = center.presented {
                        ZStack(alignment: .topLeading) {
                            Color.black.opacity(0.001)
                                .contentShape(Rectangle())
                                .onTapGesture { center.dismiss() }
                            MenuPanel(items: p.items) { center.dismiss() }
                                .fixedSize()
                                .alignmentGuide(.leading) { d in
                                    let x = p.alignTrailing ? p.anchor.maxX - d.width : p.anchor.minX
                                    return -min(max(8, x), geo.size.width - d.width - 8)
                                }
                                .alignmentGuide(.top) { d in
                                    let below = p.anchor.maxY + 6
                                    let y = below + d.height > geo.size.height - 8 ? p.anchor.minY - d.height - 6 : below
                                    return -y
                                }
                                .transition(.scale(scale: 0.96, anchor: p.alignTrailing ? .topTrailing : .topLeading).combined(with: .opacity))
                        }
                    }
                }
            }
            #if os(macOS)
            .onExitCommand { center.dismiss() }
            #endif
    }
}

private struct MenuPanel: View {
    let items: [PulseMenuItem]
    let dismiss: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            ForEach(items) { item in
                MenuRow(item: item) {
                    dismiss()
                    item.action()
                }
            }
        }
        .padding(6)
        .frame(minWidth: 220)
        .background(RoundedRectangle(cornerRadius: 14, style: .continuous).fill(Pulse.elevated))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Pulse.hairlineStrong))
        .shadow(color: .black.opacity(0.6), radius: 30, y: 16)
        .shadow(color: Pulse.accent.opacity(0.06), radius: 40)
        .accessibilityElement(children: .contain)
        .accessibilityAddTraits(.isModal)
    }
}

private struct MenuRow: View {
    let item: PulseMenuItem
    let action: () -> Void
    @State private var hovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 12) {
                Image(systemName: item.systemImage)
                    .font(.system(size: 14, weight: .medium))
                    .frame(width: 18)
                    .foregroundStyle(item.destructive ? Color(hex: 0xFF8A7A) : (hovering ? Pulse.accent : Pulse.textSecondary))
                Text(item.title)
                    .font(.pulseBody(14, weight: .medium))
                    .foregroundStyle(item.destructive ? Color(hex: 0xFF8A7A) : Pulse.text)
                Spacer(minLength: 16)
                if let shortcut = item.shortcut {
                    Text(shortcut).font(.pulseBody(12)).foregroundStyle(Pulse.textTertiary)
                }
            }
            .padding(.horizontal, 10)
            .frame(height: 36)
            .background(RoundedRectangle(cornerRadius: 9, style: .continuous).fill(hovering ? Pulse.accentPill : .clear))
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .onHover { hovering = $0 }
    }
}

extension View {
    /// Install once at the root of each window/scene.
    func pulseMenuHost() -> some View { modifier(PulseMenuHost()) }
}

/// A trigger that opens a custom Pulse menu below itself.
struct PulseMenuButton<Label: View>: View {
    let items: [PulseMenuItem]
    var alignTrailing = true
    var accessibilityLabel: String
    @ViewBuilder var label: () -> Label
    @Environment(PulseMenuCenter.self) private var center
    @State private var frame: CGRect = .zero

    var body: some View {
        Button {
            if center.presented == nil {
                center.show(items, from: frame, alignTrailing: alignTrailing)
            } else {
                center.dismiss()
            }
        } label: {
            label()
        }
        .background(GeometryReader { geo in
            Color.clear
                .onAppear { frame = geo.frame(in: .named(pulseRootSpace)) }
                .onChange(of: geo.frame(in: .named(pulseRootSpace))) { _, new in frame = new }
        })
        .accessibilityLabel(accessibilityLabel)
    }
}

/// Standard "•••" more button wired to a Pulse menu.
struct MoreMenuButton: View {
    let items: [PulseMenuItem]
    var size: CGFloat = 40

    var body: some View {
        PulseMenuButton(items: items, accessibilityLabel: "More options") {
            Image(systemName: "ellipsis")
                .font(.system(size: 16, weight: .bold))
                .foregroundStyle(Pulse.textSecondary)
                .frame(width: size, height: size)
        }
        .pulse(.ghost, shape: .circle)
    }
}
