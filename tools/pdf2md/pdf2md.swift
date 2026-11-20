import Foundation
import PDFKit
import Vision
import AppKit

// pdf2md: 用 macOS 原生 PDFKit 抽取数字文本；无文字层的页面用 Vision OCR。
// 输出 Markdown（按页分节）到 stdout；诊断信息写 stderr。

func stderr(_ message: String) {
    if let data = (message + "\n").data(using: .utf8) {
        FileHandle.standardError.write(data)
    }
}

func ocrText(from image: CGImage) throws -> String {
    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = true
    request.recognitionLanguages = ["zh-Hans", "en-US"]

    let handler = VNImageRequestHandler(cgImage: image, options: [:])
    try handler.perform([request])

    guard let observations = request.results else { return "" }
    let lines: [(CGFloat, CGFloat, String)] = observations.compactMap { obs in
        guard let candidate = obs.topCandidates(1).first else { return nil }
        let box = obs.boundingBox
        // Vision 的坐标原点在左下；-midY 让页面从上到下排序。
        return (-box.midY, box.minX, candidate.string)
    }.sorted { lhs, rhs in
        if abs(lhs.0 - rhs.0) > 0.012 { return lhs.0 < rhs.0 }
        return lhs.1 < rhs.1
    }
    return lines.map { $0.2 }.joined(separator: "\n")
}

func render(page: PDFPage, scale: CGFloat) -> CGImage? {
    let bounds = page.bounds(for: .mediaBox)
    let width = Int((bounds.width * scale).rounded())
    let height = Int((bounds.height * scale).rounded())
    guard width > 0, height > 0 else { return nil }
    guard let context = CGContext(
        data: nil,
        width: width,
        height: height,
        bitsPerComponent: 8,
        bytesPerRow: 0,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue
    ) else { return nil }

    context.setFillColor(NSColor.white.cgColor)
    context.fill(CGRect(x: 0, y: 0, width: width, height: height))
    context.scaleBy(x: scale, y: scale)
    page.draw(with: .mediaBox, to: context)
    return context.makeImage()
}

func main() -> Int32 {
    let args = CommandLine.arguments
    guard args.count >= 2 else {
        stderr("用法: pdf2md <input.pdf>")
        return 1
    }
    let url = URL(fileURLWithPath: args[1])
    guard let document = PDFDocument(url: url) else {
        stderr("无法打开 PDF: \(url.path)")
        return 2
    }
    guard document.pageCount > 0 else {
        stderr("PDF 没有页面: \(url.path)")
        return 2
    }

    print("# \(url.lastPathComponent)\n")
    var ocrPages = 0
    for index in 0..<document.pageCount {
        autoreleasepool {
            guard let page = document.page(at: index) else { return }
            let native = (page.string ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            var text = native
            if native.count < 10 {
                if let image = render(page: page, scale: 2.0),
                   let recognized = try? ocrText(from: image) {
                    text = recognized.trimmingCharacters(in: .whitespacesAndNewlines)
                    if !text.isEmpty { ocrPages += 1 }
                }
            }
            print("## 第 \(index + 1) 页\n")
            print(text.isEmpty ? "（本页无可提取文字）" : text)
            print("\n")
        }
    }
    stderr("页数: \(document.pageCount)，OCR 页数: \(ocrPages)")
    return 0
}

exit(main())
