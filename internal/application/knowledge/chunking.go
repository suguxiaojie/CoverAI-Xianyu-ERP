package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// maxChunkRunes 是第一阶段文本分块的最大 Unicode 字符数，不依赖具体模型 tokenizer。
const maxChunkRunes = 500

// BuildChunks 按 FAQ 整体或文档段落确定性切分内容并生成 SHA-256 摘要。
func BuildChunks(draft EntryDraft) []Chunk {
	if draft.Type == ContentTypeFAQ {
		// content 是 FAQ 问题与标准答案组成的单一证据片段。
		content := strings.TrimSpace(draft.Title + "\n" + draft.Content)
		return []Chunk{{Index: 0, Content: content, ContentHash: ContentDigest(content)}}
	}
	// paragraphs 是按空行切分并去除空白的文档段落。
	paragraphs := normalizedParagraphs(draft.Content)
	// chunks 是按段落边界组合后的最终分块。
	chunks := make([]Chunk, 0, len(paragraphs))
	// current 是尚未达到分块上限的累积文本。
	current := ""
	// paragraph 是当前待合并或独立切分的文档段落。
	for _, paragraph := range paragraphs {
		if utf8.RuneCountInString(paragraph) > maxChunkRunes {
			if current != "" {
				chunks = appendChunk(chunks, current)
				current = ""
			}
			// part 是超长段落按 Unicode 字符上限切分后的一段。
			for _, part := range splitRunes(paragraph, maxChunkRunes) {
				chunks = appendChunk(chunks, part)
			}
			continue
		}
		// candidate 是当前累积块与新段落组成的待检查文本。
		candidate := paragraph
		if current != "" {
			candidate = current + "\n\n" + paragraph
		}
		if utf8.RuneCountInString(candidate) <= maxChunkRunes {
			current = candidate
			continue
		}
		chunks = appendChunk(chunks, current)
		current = paragraph
	}
	if current != "" {
		chunks = appendChunk(chunks, current)
	}
	return chunks
}

// ContentDigest 返回内容的小写十六进制 SHA-256 摘要，用于去重和变更判断。
func ContentDigest(content string) string {
	// digest 是对 UTF-8 内容计算得到的 32 字节 SHA-256 值。
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

// normalizedParagraphs 按空行切分文档并去除空段落。
func normalizedParagraphs(content string) []string {
	// normalized 统一 Windows 与旧 Mac 换行，使分块结果跨平台稳定。
	normalized := strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	// rawParagraphs 是按两个换行切分的原始段落。
	rawParagraphs := strings.Split(normalized, "\n\n")
	// paragraphs 是去除首尾空白和空段落后的结果。
	paragraphs := make([]string, 0, len(rawParagraphs))
	// paragraph 是当前待规范化的原始段落。
	for _, paragraph := range rawParagraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return paragraphs
}

// splitRunes 在不截断 UTF-8 字符的前提下按固定 Unicode 字符数切分文本。
func splitRunes(content string, limit int) []string {
	// runes 是待切分文本的 Unicode 字符序列。
	runes := []rune(content)
	// parts 是按 limit 切分后的文本片段。
	parts := make([]string, 0, (len(runes)+limit-1)/limit)
	// start 是当前片段在 Unicode 序列中的开始下标。
	for start := 0; start < len(runes); start += limit {
		// end 是当前片段的排除结束下标。
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, strings.TrimSpace(string(runes[start:end])))
	}
	return parts
}

// appendChunk 将非空文本追加为带稳定顺序和内容摘要的分块。
func appendChunk(chunks []Chunk, content string) []Chunk {
	content = strings.TrimSpace(content)
	if content == "" {
		return chunks
	}
	return append(chunks, Chunk{Index: len(chunks), Content: content, ContentHash: ContentDigest(content)})
}
