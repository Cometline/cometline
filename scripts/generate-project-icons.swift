import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

struct Output {
	let path: String
	let size: Int
	let radius: CGFloat
	/// How much of the canvas the artwork should fill, centered (1.0 = full bleed).
	let artworkScale: CGFloat
	/// When false, macOS applies the Dock squircle mask itself.
	let clipToRoundedRect: Bool
}

func fail(_ message: String) -> Never {
	fputs("error: \(message)\n", stderr)
	exit(1)
}

func run(_ executable: String, _ arguments: [String]) {
	let process = Process()
	process.executableURL = URL(fileURLWithPath: executable)
	process.arguments = arguments

	do {
		try process.run()
		process.waitUntilExit()
	} catch {
		fail("failed to run \(executable): \(error.localizedDescription)")
	}

	if process.terminationStatus != 0 {
		fail("\(executable) exited with status \(process.terminationStatus)")
	}
}

func commandPath(_ name: String) -> String? {
	let process = Process()
	process.executableURL = URL(fileURLWithPath: "/usr/bin/which")
	process.arguments = [name]

	let pipe = Pipe()
	process.standardOutput = pipe
	process.standardError = Pipe()

	try? process.run()
	process.waitUntilExit()

	guard process.terminationStatus == 0 else { return nil }
	let data = pipe.fileHandleForReading.readDataToEndOfFile()
	return String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines)
}

let fileManager = FileManager.default
let cwd = fileManager.currentDirectoryPath
let sourceCandidates = ["../project_icon.png", "static/project_icon.png"]
guard let sourcePath = sourceCandidates.first(where: { fileManager.fileExists(atPath: $0) }) else {
	fail("missing source image; expected ../project_icon.png or static/project_icon.png from \(cwd)")
}

let outputs = [
	Output(path: "static/project_avatar_96.png", size: 96, radius: 48, artworkScale: 1.0, clipToRoundedRect: true),
	Output(path: "static/project_avatar_192.png", size: 192, radius: 96, artworkScale: 1.0, clipToRoundedRect: true),
	Output(path: "static/project_avatar_384.png", size: 384, radius: 192, artworkScale: 1.0, clipToRoundedRect: true),
	// Dock icons: inset to Apple's 832×832 safe zone and bake the squircle mask into
	// the PNG. Electron's app.dock.setIcon() does not apply macOS masking.
	Output(path: "static/app_icon.png", size: 1024, radius: 224, artworkScale: 0.8125, clipToRoundedRect: true),
	Output(path: "buildResources/icon.png", size: 1024, radius: 224, artworkScale: 0.8125, clipToRoundedRect: true)
]

guard let source = CGImageSourceCreateWithURL(URL(fileURLWithPath: sourcePath) as CFURL, nil),
	let sourceImage = CGImageSourceCreateImageAtIndex(source, 0, nil)
else {
	fail("could not read \(sourcePath)")
}

let cropSize = min(sourceImage.width, sourceImage.height)
let cropX = (sourceImage.width - cropSize) / 2
let cropY = (sourceImage.height - cropSize) / 2
guard let cropped = sourceImage.cropping(to: CGRect(x: cropX, y: cropY, width: cropSize, height: cropSize)) else {
	fail("could not crop source image")
}

let colorSpace = CGColorSpaceCreateDeviceRGB()
let bitmapInfo = CGImageAlphaInfo.premultipliedLast.rawValue

for output in outputs {
	guard let context = CGContext(
		data: nil,
		width: output.size,
		height: output.size,
		bitsPerComponent: 8,
		bytesPerRow: 0,
		space: colorSpace,
		bitmapInfo: bitmapInfo
	) else {
		fail("could not create context for \(output.path)")
	}

	let rect = CGRect(x: 0, y: 0, width: output.size, height: output.size)
	context.interpolationQuality = .high
	if output.clipToRoundedRect {
		let inset = CGFloat(output.size) * (1 - output.artworkScale) / 2
		let clipRect = output.artworkScale < 1.0 ? rect.insetBy(dx: inset, dy: inset) : rect
		let cornerRadius = output.artworkScale < 1.0 ? output.radius * output.artworkScale : output.radius
		context.addPath(
			CGPath(
				roundedRect: clipRect,
				cornerWidth: cornerRadius,
				cornerHeight: cornerRadius,
				transform: nil
			)
		)
		context.clip()
		context.draw(cropped, in: clipRect)
	} else {
		let artworkSize = CGFloat(output.size) * output.artworkScale
		let artworkOffset = (CGFloat(output.size) - artworkSize) / 2
		let artworkRect = CGRect(
			x: artworkOffset,
			y: artworkOffset,
			width: artworkSize,
			height: artworkSize
		)
		context.draw(cropped, in: artworkRect)
	}

	guard let image = context.makeImage() else {
		fail("could not render \(output.path)")
	}

	let url = URL(fileURLWithPath: output.path)
	try? fileManager.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
	guard let destination = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil) else {
		fail("could not create destination for \(output.path)")
	}
	CGImageDestinationAddImage(destination, image, nil)
	guard CGImageDestinationFinalize(destination) else {
		fail("could not write \(output.path)")
	}
}

guard let sips = commandPath("sips"), let iconutil = commandPath("iconutil") else {
	fail("sips and iconutil are required to generate buildResources/icon.icns")
}

let iconset = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("cometline.iconset")
try? fileManager.removeItem(at: iconset)
try? fileManager.createDirectory(at: iconset, withIntermediateDirectories: true)

let iconPng = "buildResources/icon.png"
let iconSizes = [
	("icon_16x16.png", 16),
	("icon_16x16@2x.png", 32),
	("icon_32x32.png", 32),
	("icon_32x32@2x.png", 64),
	("icon_128x128.png", 128),
	("icon_128x128@2x.png", 256),
	("icon_256x256.png", 256),
	("icon_256x256@2x.png", 512),
	("icon_512x512.png", 512)
]

for (name, size) in iconSizes {
	run(sips, ["-z", String(size), String(size), iconPng, "--out", iconset.appendingPathComponent(name).path])
}

try? fileManager.copyItem(atPath: iconPng, toPath: iconset.appendingPathComponent("icon_512x512@2x.png").path)
run(iconutil, ["-c", "icns", iconset.path, "-o", "buildResources/icon.icns"])

print("Generated project avatar and app icon assets from \(sourcePath).")
