import SwiftUI

// Every interactive control in AudioPulse is drawn here. No stock SwiftUI
// button, menu, picker, segmented control, slider or toggle chrome is used.

// MARK: - Buttons

enum PulseButtonKind {
    /// Mint fill, dark glyph: play/pause, primary actions.
    case primary
    /// Hairline outline pill: secondary actions ("Smart shuffle").
    case outline
    /// Transparent until hovered/pressed: icon buttons, sidebar rows.
    case ghost
    /// Soft filled chip: prompt suggestions, filters.
    case chip
    /// Tinted accent pill: "liked" heart, selected state.
    case tinted
}

struct PulseButtonStyle: ButtonStyle {
    var kind: PulseButtonKind = .ghost
    var shape: PulseShape = .capsule

    func makeBody(configuration: Configuration) -> some View {
        PulseButtonBody(configuration: configuration, kind: kind, shape: shape)
    }
}

enum PulseShape {
    case capsule, circle, rounded(CGFloat)
}

private struct PulseButtonBody: View {
    let configuration: ButtonStyleConfiguration
    let kind: PulseButtonKind
    let shape: PulseShape
    @State private var hovering = false
    @Environment(\.isEnabled) private var enabled
    @Environment(\.isFocused) private var focused

    var body: some View {
        configuration.label
            .foregroundStyle(foreground)
            .background(filled(background))
            .overlay(stroked(border, 1, inset: 0))
            .overlay(stroked(focused ? Pulse.accent : .clear, 2, inset: -3))
            .contentShape(Rectangle())
            .scaleEffect(configuration.isPressed ? 0.95 : 1)
            .brightness(kind == .primary && hovering ? 0.06 : 0)
            .opacity(enabled ? 1 : 0.4)
            .animation(.spring(response: 0.25, dampingFraction: 0.7), value: configuration.isPressed)
            .animation(.easeOut(duration: 0.12), value: hovering)
            .onHover { hovering = $0 }
            #if os(macOS)
            .focusEffectDisabled()
            #endif
    }

    @ViewBuilder private func filled(_ color: Color) -> some View {
        switch shape {
        case .capsule: Capsule().fill(color)
        case .circle: Circle().fill(color)
        case .rounded(let r): RoundedRectangle(cornerRadius: r, style: .continuous).fill(color)
        }
    }

    @ViewBuilder private func stroked(_ color: Color, _ width: CGFloat, inset: CGFloat) -> some View {
        switch shape {
        case .capsule: Capsule().strokeBorder(color, lineWidth: width).padding(inset)
        case .circle: Circle().strokeBorder(color, lineWidth: width).padding(inset)
        case .rounded(let r): RoundedRectangle(cornerRadius: r, style: .continuous).strokeBorder(color, lineWidth: width).padding(inset)
        }
    }

    private var foreground: Color {
        switch kind {
        case .primary: Pulse.onAccent
        case .tinted: Pulse.accent
        default: Pulse.text
        }
    }

    private var background: Color {
        let pressed = configuration.isPressed
        switch kind {
        case .primary: return Pulse.accent
        case .outline: return hovering || pressed ? Pulse.fill : .clear
        case .ghost: return pressed ? Pulse.fillStrong : (hovering ? Pulse.fill : .clear)
        case .chip: return pressed ? Pulse.fillStrong : (hovering ? Pulse.fillStrong : Pulse.fill)
        case .tinted: return hovering || pressed ? Pulse.accent.opacity(0.2) : Pulse.accentPill
        }
    }

    private var border: Color {
        switch kind {
        case .outline: hovering ? Pulse.hairlineStrong.opacity(1.6) : Pulse.hairlineStrong
        case .chip: Pulse.hairline
        default: .clear
        }
    }
}

extension View {
    func pulse(_ kind: PulseButtonKind = .ghost, shape: PulseShape = .capsule) -> some View {
        buttonStyle(PulseButtonStyle(kind: kind, shape: shape))
    }
}

/// A square/circular icon button with a 44pt hit target.
struct IconButton: View {
    let systemName: String
    let label: String
    var size: CGFloat = 44
    var glyph: CGFloat = 17
    var kind: PulseButtonKind = .ghost
    var tint: Color? = nil
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemName)
                .font(.system(size: glyph, weight: .semibold))
                .foregroundStyle(tint ?? (kind == .primary ? Pulse.onAccent : Pulse.text))
                .frame(width: size, height: size)
        }
        .pulse(kind, shape: .circle)
        .accessibilityLabel(label)
    }
}

// MARK: - Segmented control

/// Pill segmented control with a sliding mint indicator.
struct PulseSegmented<Value: Hashable>: View {
    let options: [(Value, String)]
    @Binding var selection: Value
    @Namespace private var ns

    var body: some View {
        HStack(spacing: 4) {
            ForEach(options, id: \.0) { value, title in
                let selected = value == selection
                Button {
                    withAnimation(.spring(response: 0.3, dampingFraction: 0.8)) { selection = value }
                } label: {
                    Text(title)
                        .font(.pulseBody(13, weight: selected ? .semibold : .medium))
                        .foregroundStyle(selected ? Pulse.onAccent : Pulse.text)
                        .padding(.horizontal, 16)
                        .frame(height: 34)
                        .background {
                            if selected {
                                Capsule().fill(Pulse.accent).matchedGeometryEffect(id: "seg", in: ns)
                            }
                        }
                        .contentShape(Capsule())
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityAddTraits(selected ? .isSelected : [])
            }
        }
        .padding(4)
        .background(Capsule().fill(Pulse.fill))
    }
}

/// Filter chips (Ask palette tabs). Selected chip turns tinted mint.
struct PulseChips<Value: Hashable>: View {
    let options: [(Value, String)]
    @Binding var selection: Value

    var body: some View {
        HStack(spacing: 6) {
            ForEach(options, id: \.0) { value, title in
                let selected = value == selection
                Button { selection = value } label: {
                    Text(title)
                        .font(.pulseBody(13, weight: .medium))
                        .foregroundStyle(selected ? Pulse.accent : Pulse.text)
                        .padding(.horizontal, 12)
                        .frame(height: 30)
                        .background(Capsule().fill(selected ? Pulse.accentPill : .clear))
                        .overlay(Capsule().strokeBorder(selected ? .clear : Pulse.hairlineStrong))
                        .contentShape(Capsule())
                }
                .buttonStyle(PlainPressStyle())
                .accessibilityAddTraits(selected ? .isSelected : [])
            }
        }
    }
}

/// Press feedback only, for controls that draw their own chrome.
struct PlainPressStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.96 : 1)
            .animation(.spring(response: 0.25, dampingFraction: 0.7), value: configuration.isPressed)
            #if os(macOS)
            .focusEffectDisabled()
            #endif
    }
}

// MARK: - Slider

/// Thin custom slider (volume). Grows a thumb on hover/drag.
struct PulseSlider: View {
    @Binding var value: Double
    var tint: Color = Pulse.text
    var label: String
    @State private var active = false

    var body: some View {
        GeometryReader { geo in
            let w = geo.size.width
            ZStack(alignment: .leading) {
                Capsule().fill(Color.white.opacity(0.14)).frame(height: 4)
                Capsule().fill(active ? Pulse.accent : tint).frame(width: max(4, w * value), height: 4)
                Circle()
                    .fill(Pulse.text)
                    .frame(width: active ? 12 : 0, height: active ? 12 : 0)
                    .offset(x: w * value - (active ? 6 : 0))
            }
            .frame(maxHeight: .infinity)
            .contentShape(Rectangle())
            .gesture(DragGesture(minimumDistance: 0)
                .onChanged { g in
                    active = true
                    value = min(1, max(0, g.location.x / w))
                }
                .onEnded { _ in active = false })
            .onHover { active = $0 }
            .animation(.easeOut(duration: 0.12), value: active)
        }
        .frame(height: 20)
        .accessibilityElement()
        .accessibilityLabel(label)
        .accessibilityValue("\(Int(value * 100)) percent")
        .accessibilityAdjustableAction { dir in
            switch dir {
            case .increment: value = min(1, value + 0.05)
            case .decrement: value = max(0, value - 0.05)
            @unknown default: break
            }
        }
    }
}

// MARK: - Toggle

struct PulseToggle: View {
    let title: String
    @Binding var isOn: Bool

    var body: some View {
        Button { withAnimation(.spring(response: 0.3, dampingFraction: 0.75)) { isOn.toggle() } } label: {
            HStack {
                Text(title).font(.pulseBody(14, weight: .medium)).foregroundStyle(Pulse.text)
                Spacer()
                ZStack(alignment: isOn ? .trailing : .leading) {
                    Capsule().fill(isOn ? Pulse.accent : Pulse.fillStrong).frame(width: 42, height: 24)
                    Circle().fill(isOn ? Pulse.onAccent : Pulse.textSecondary).frame(width: 18, height: 18).padding(3)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(PlainPressStyle())
        .accessibilityLabel(title)
        .accessibilityValue(isOn ? "On" : "Off")
    }
}

// MARK: - Badges

struct PulseBadge: View {
    let text: String
    var filled = true

    var body: some View {
        Text(text)
            .font(.system(size: 10, weight: .bold))
            .tracking(0.6)
            .padding(.horizontal, 6)
            .padding(.vertical, 3)
            .foregroundStyle(filled ? Pulse.onAccent : Pulse.text)
            .background(RoundedRectangle(cornerRadius: 5).fill(filled ? Pulse.accent : Color.white.opacity(0.12)))
    }
}

struct UnreadDot: View {
    var body: some View {
        Circle().fill(Pulse.accent).frame(width: 7, height: 7)
    }
}
