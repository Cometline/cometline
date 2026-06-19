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

struct Variant {
	let label: String
	let sourceCandidates: [String]
	/// When set, writes the center-cropped square master at full resolution.
	let masterOutput: String?
	let outputs: [Output]
	let generateIcns: Bool
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

func centerCroppedSquare(from sourceImage: CGImage) -> CGImage? {
	let cropSize = min(sourceImage.width, sourceImage.height)
	let cropX = (sourceImage.width - cropSize) / 2
	let cropY = (sourceImage.height - cropSize) / 2
	return sourceImage.cropping(to: CGRect(x: cropX, y: cropY, width: cropSize, height: cropSize))
}

func writePNG(_ image: CGImage, to path: String) {
	let fileManager = FileManager.default
	let url = URL(fileURLWithPath: path)
	try? fileManager.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
	guard let destination = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil) else {
		fail("could not create destination for \(path)")
	}
	CGImageDestinationAddImage(destination, image, nil)
	guard CGImageDestinationFinalize(destination) else {
		fail("could not write \(path)")
	}
}

func renderOutput(_ output: Output, from cropped: CGImage) {
	let colorSpace = CGColorSpaceCreateDeviceRGB()
	let bitmapInfo = CGImageAlphaInfo.premultipliedLast.rawValue

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
	writePNG(image, to: output.path)
}

func generateIcns(from iconPng: String) {
	guard let sips = commandPath("sips"), let iconutil = commandPath("iconutil") else {
		fail("sips and iconutil are required to generate buildResources/icon.icns")
	}

	let fileManager = FileManager.default
	let iconset = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent("cometline.iconset")
	try? fileManager.removeItem(at: iconset)
	try? fileManager.createDirectory(at: iconset, withIntermediateDirectories: true)

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
}

let avatarOutputs = [
	Output(path: "static/project_avatar_96.png", size: 96, radius: 48, artworkScale: 1.0, clipToRoundedRect: true),
	Output(path: "static/project_avatar_192.png", size: 192, radius: 96, artworkScale: 1.0, clipToRoundedRect: true),
	Output(path: "static/project_avatar_384.png", size: 384, radius: 192, artworkScale: 1.0, clipToRoundedRect: true)
]

let dockOutput = Output(
	path: "static/app_icon.png",
	size: 1024,
	radius: 224,
	artworkScale: 0.8125,
	clipToRoundedRect: true
)

let variants: [Variant] = [
	Variant(
		label: "default",
		sourceCandidates: ["../project_icon.png", "static/project_icon.png"],
		masterOutput: nil,
		outputs: avatarOutputs + [
			dockOutput,
			Output(path: "buildResources/icon.png", size: 1024, radius: 224, artworkScale: 0.8125, clipToRoundedRect: true)
		],
		generateIcns: true
	),
	Variant(
		label: "man",
		sourceCandidates: ["static/app_icon_man.png", "static/project_icon_man.png"],
		masterOutput: "static/project_icon_man.png",
		outputs: [
			Output(path: "static/project_avatar_man_96.png", size: 96, radius: 48, artworkScale: 1.0, clipToRoundedRect: true),
			Output(path: "static/project_avatar_man_192.png", size: 192, radius: 96, artworkScale: 1.0, clipToRoundedRect: true),
			Output(path: "static/project_avatar_man_384.png", size: 384, radius: 192, artworkScale: 1.0, clipToRoundedRect: true),
			Output(path: "static/app_icon_man.png", size: 1024, radius: 224, artworkScale: 0.8125, clipToRoundedRect: true)
		],
		generateIcns: false
	)
]

let requestedVariant = CommandLine.arguments.dropFirst().first
let selectedVariants = variants.filter { variant in
	guard let requestedVariant else { return true }
	return variant.label == requestedVariant
}

guard !selectedVariants.isEmpty else {
	let available = variants.map(\.label).joined(separator: ", ")
	fail("unknown variant \(requestedVariant ?? ""); expected one of: \(available)")
}

let fileManager = FileManager.default

for variant in selectedVariants {
	guard let sourcePath = variant.sourceCandidates.first(where: { fileManager.fileExists(atPath: $0) }) else {
		fail("missing source image for \(variant.label); expected one of: \(variant.sourceCandidates.joined(separator: ", "))")
	}

	guard let source = CGImageSourceCreateWithURL(URL(fileURLWithPath: sourcePath) as CFURL, nil),
		let sourceImage = CGImageSourceCreateImageAtIndex(source, 0, nil),
		let cropped = centerCroppedSquare(from: sourceImage)
	else {
		fail("could not read or crop \(sourcePath)")
	}

	if let masterOutput = variant.masterOutput {
		writePNG(cropped, to: masterOutput)
	}

	for output in variant.outputs {
		renderOutput(output, from: cropped)
	}

	if variant.generateIcns {
		generateIcns(from: "buildResources/icon.png")
	}

	if variant.label == "man" {
		generateTrayIcons(from: "static/app_icon_man.png", baseName: "trayIcon_man")
	}

	print("Generated \(variant.label) project avatar and app icon assets from \(sourcePath).")
}

func generateTrayIcons(from dockIconPath: String, baseName: String) {
	guard let sips = commandPath("sips") else {
		fail("sips is required to generate \(baseName) tray icons")
	}
	let cropPath = URL(fileURLWithPath: NSTemporaryDirectory())
		.appendingPathComponent("cometline-tray-crop-\(baseName).png").path
	run(sips, ["-c", "850", "850", dockIconPath, "--out", cropPath])
	run(sips, ["-z", "16", "16", cropPath, "--out", "buildResources/\(baseName).png"])
	run(sips, ["-z", "32", "32", cropPath, "--out", "buildResources/\(baseName)@2x.png"])
}
