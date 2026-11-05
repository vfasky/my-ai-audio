// Renders the firmware caption "伊言" to a transparent PNG (macOS only):
//   swift tools/render-brand-caption.swift assets/brand/yiyan-caption.png
import AppKit

let out = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "assets/brand/yiyan-caption.png"
let width = 56, height = 28
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: width, pixelsHigh: height, bitsPerSample: 8,
                           samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                           bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
let font = NSFont(name: "PingFangSC-Semibold", size: 22) ?? NSFont.boldSystemFont(ofSize: 22)
let attrs: [NSAttributedString.Key: Any] = [
  .font: font, .foregroundColor: NSColor.white, .kern: 4,
]
let text = NSAttributedString(string: "伊言", attributes: attrs)
let size = text.size()
text.draw(at: NSPoint(x: (CGFloat(width) - size.width + 4) / 2, y: (CGFloat(height) - size.height) / 2))
NSGraphicsContext.restoreGraphicsState()
try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: out))
