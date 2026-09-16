package knowledge

import "strings"

// searchSynonymGroup 定义只用于检索的高确定性中文表达归一化。
type searchSynonymGroup struct {
	// Canonical 是同义表达统一后的稳定文本。
	Canonical string
	// Variants 是从长到短匹配的常见中文问法。
	Variants []string
}

// searchSynonymGroups 是不涉及业务承诺的通用中文同义表达集合。
var searchSynonymGroups = []searchSynonymGroup{
	{Canonical: "如何", Variants: []string{"怎么样", "怎样", "怎么", "咋样", "咋"}},
	{Canonical: "买", Variants: []string{"购买", "下单", "拍下"}},
	{Canonical: "哪个", Variants: []string{"哪一个", "哪款"}},
	{Canonical: "价格", Variants: []string{"多少钱", "什么价格", "什么价"}},
	{Canonical: "库存", Variants: []string{"有库存", "有货", "能拍"}},
}

// normalizeSearchPhrase 将标点清理后的通用同义表达归一化，不处理业务专有名词。
func normalizeSearchPhrase(value string) string {
	// normalized 是去除标点、空白并转小写后的检索文本。
	normalized := normalizeSearchText(value)
	// group 是当前待应用的高确定性同义表达组。
	for _, group := range searchSynonymGroups {
		// variant 是当前待替换为稳定表达的常见问法。
		for _, variant := range group.Variants {
			normalized = strings.ReplaceAll(normalized, variant, group.Canonical)
		}
	}
	return normalized
}

// buildAliasSuggestions 根据标准问题和少量业务主题生成最多八条不落库建议。
func buildAliasSuggestions(faq Entry, aliases []FAQAlias) []string {
	// candidates 保存标准问题变体和明确主题模板。
	candidates := make([]string, 0, 12)
	// title 是去除首尾空白后的 FAQ 标准问题。
	title := strings.TrimSpace(faq.Title)
	if strings.Contains(title, "如何") {
		candidates = append(candidates, strings.ReplaceAll(title, "如何", "怎么"), strings.ReplaceAll(title, "如何", "怎样"))
	}
	if strings.Contains(title, "怎么") {
		candidates = append(candidates, strings.ReplaceAll(title, "怎么", "如何"), strings.ReplaceAll(title, "怎么", "怎样"))
	}
	// searchableText 是用于识别 FAQ 明确业务主题的标题和正文。
	searchableText := title + faq.Content
	if strings.Contains(searchableText, "充值") || strings.Contains(searchableText, "兑换") {
		candidates = append(candidates, "怎么充值", "如何充值", "怎样充值", "充值流程是什么", "购买后怎么操作", "付款后怎么充值")
	}
	if strings.Contains(searchableText, "套餐") || strings.Contains(searchableText, "额度") {
		candidates = append(candidates, "有什么套餐", "套餐怎么选", "应该选哪个套餐")
	}
	if strings.Contains(searchableText, "库存") || strings.Contains(searchableText, "有货") {
		candidates = append(candidates, "现在有货吗", "还有库存吗", "现在能拍吗")
	}
	if strings.Contains(searchableText, "价格") || strings.Contains(searchableText, "标价") {
		candidates = append(candidates, "多少钱", "现在什么价格", "页面价格是多少")
	}
	// excluded 保存标准问题和既有别名的规范化文本，避免重复建议。
	excluded := map[string]struct{}{normalizeSearchPhrase(title): {}}
	// alias 是当前待排除的既有相似问法。
	for _, alias := range aliases {
		excluded[normalizeSearchPhrase(alias.Alias)] = struct{}{}
	}
	// result 是按生成顺序去重后的最多八条建议。
	result := make([]string, 0, 8)
	// candidate 是当前待规范化、去重和限制数量的建议。
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		// normalizedCandidate 是建议用于去重的同义词归一化文本。
		normalizedCandidate := normalizeSearchPhrase(candidate)
		if normalizedCandidate == "" {
			continue
		}
		// exists 表示当前规范化建议是否已由标准问题、既有问法或前序建议占用。
		if _, exists := excluded[normalizedCandidate]; exists {
			continue
		}
		excluded[normalizedCandidate] = struct{}{}
		result = append(result, candidate)
		if len(result) >= 8 {
			break
		}
	}
	return result
}
