package knowledge

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// highRiskTerms 是第一阶段必须绕过候选回复并转人工的稳定业务词。
var highRiskTerms = []string{"退款", "投诉", "补发", "卡密", "验证码", "cookie", "token", "密码", "风控", "封号"}

// rankedEntry 保存一条已审核知识与确定性文本相关性分数。
type rankedEntry struct {
	// entry 是当前排名使用的知识条目。
	entry Entry
	// score 是 0 到 1 之间的标题、正文和二元字符匹配结果。
	score float64
}

// Retrieve 在不调用外部模型的前提下执行文本检索、风险门禁和脱敏日志。
func (service *Service) Retrieve(ctx context.Context, request RetrieveRequest) (RetrieveResult, error) {
	// validationErr 是离线检索前的服务依赖和用户身份校验结果。
	if validationErr := service.validateUser(request.UserID); validationErr != nil {
		return RetrieveResult{}, validationErr
	}
	// query 是去除首尾空白后的离线调试问题。
	query := strings.TrimSpace(request.Query)
	if query == "" || len([]rune(query)) > 1000 {
		return RetrieveResult{}, &ValidationError{Message: "调试问题必须为 1 到 1000 个字符"}
	}
	// knowledgeBaseIDs 是去重且只保留正数的用户显式调试范围。
	knowledgeBaseIDs := uniquePositiveIDs(request.KnowledgeBaseIDs)
	// candidates 是当前时间有效的已审核、已启用知识条目。
	candidates, listErr := service.repository.ListRetrievableEntries(ctx, request.UserID, knowledgeBaseIDs, currentUnix())
	if listErr != nil {
		return RetrieveResult{}, listErr
	}
	// result 是将被写入脱敏日志并返回 HTTP 层的门禁结果。
	result := buildRetrieveResult(query, candidates)
	// logErr 是仅写入问题摘要与结论的审计日志错误；日志不可用时拒绝伪装成可审计成功。
	logErr := service.repository.AddRetrieveLog(ctx, request.UserID, ContentDigest(query), result.Answerability, len(result.Evidence))
	if logErr != nil {
		return RetrieveResult{}, logErr
	}
	return result, nil
}

// buildRetrieveResult 将高风险词、文本排名和实时数据需求组合成确定性门禁结果。
func buildRetrieveResult(query string, candidates []Entry) RetrieveResult {
	// highRisk 表示问题是否命中必须人工处理的稳定词。
	highRisk := containsAny(strings.ToLower(query), highRiskTerms)
	// ranked 是按相关性降序排列的候选知识。
	ranked := rankEntries(query, candidates)
	// evidence 是最多三条达到最低相关性的引用证据。
	evidence := evidenceFromRanked(ranked, 3)
	if highRisk {
		return RetrieveResult{Query: query, Answerability: AnswerabilityNeedsHuman, Explanation: "命中高风险意图；知识只能解释边界，不能承诺或执行退款、补发或敏感操作。", Candidate: "该问题需要人工核对实时订单和业务状态。", Evidence: evidence}
	}
	if len(ranked) == 0 || ranked[0].score < 0.22 {
		return RetrieveResult{Query: query, Answerability: AnswerabilityInsufficient, Explanation: "没有找到能够直接支撑答案的已审核知识，应先补充权威内容或转人工。", Evidence: evidence}
	}
	// top 是相关性最高的已审核权威条目。
	top := ranked[0].entry
	if top.RiskLevel == RiskLevelHigh {
		return RetrieveResult{Query: query, Answerability: AnswerabilityNeedsHuman, Explanation: "最高相关证据被标记为高风险，第一阶段只允许转人工处理。", Candidate: "该问题已标记为人工处理。", Evidence: evidence}
	}
	if top.RequiresLiveData {
		return RetrieveResult{Query: query, Answerability: AnswerabilityNeedsLiveData, Explanation: "找到了知识边界，但回答依赖实时商品或订单状态；第一阶段不生成确定承诺。", Candidate: "需要读取当前订单或商品状态后才能生成候选回复。", Evidence: evidence}
	}
	return RetrieveResult{Query: query, Answerability: AnswerabilityAnswerable, Explanation: "已找到当前生效的已审核知识，最高分证据不需要实时数据且未命中高风险门禁。", Candidate: top.Content, Evidence: evidence}
}

// rankEntries 按标题完整命中、文本包含和中文／英文二元字符重合计算相关性。
func rankEntries(query string, entries []Entry) []rankedEntry {
	// normalizedQuery 是去除标点并完成高确定性同义词归一化的查询文本。
	normalizedQuery := normalizeSearchPhrase(query)
	// ranked 是达到非零相关性的候选知识。
	ranked := make([]rankedEntry, 0, len(entries))
	// entry 是当前待计算文本相关性的已审核条目。
	for _, entry := range entries {
		// normalizedTitle 是用于高权重匹配的同义词归一化标题。
		normalizedTitle := normalizeSearchPhrase(entry.Title)
		// normalizedBody 是用于正文包含和二元字符匹配的同义词归一化正文。
		normalizedBody := normalizeSearchPhrase(entry.Content)
		// score 累加标题、正文和二元字符重合得分。
		score := 0.0
		if normalizedTitle == normalizedQuery {
			score += 0.65
		} else if strings.Contains(normalizedTitle, normalizedQuery) || strings.Contains(normalizedQuery, normalizedTitle) {
			score += 0.4
		}
		if strings.Contains(normalizedBody, normalizedQuery) {
			score += 0.3
		}
		// overlap 是查询与标题／正文的二元字符重合比例。
		overlap := bigramOverlap(normalizedQuery, normalizedTitle+normalizedBody)
		score += overlap * 0.45
		// alias 是当前 FAQ 用户确认且已启用的相似问法。
		for _, alias := range entry.Aliases {
			// normalizedAlias 是完成通用同义词归一化的相似问法。
			normalizedAlias := normalizeSearchPhrase(alias)
			// aliasScore 是当前相似问法独立计算的匹配分数。
			aliasScore := 0.0
			if normalizedAlias == normalizedQuery {
				aliasScore += 0.75
			} else if strings.Contains(normalizedAlias, normalizedQuery) || strings.Contains(normalizedQuery, normalizedAlias) {
				aliasScore += 0.5
			}
			aliasScore += bigramOverlap(normalizedQuery, normalizedAlias) * 0.45
			if aliasScore > score {
				score = aliasScore
			}
		}
		if score > 0 {
			ranked = append(ranked, rankedEntry{entry: entry, score: math.Min(1, score)})
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].score == ranked[right].score {
			return ranked[left].entry.ID < ranked[right].entry.ID
		}
		return ranked[left].score > ranked[right].score
	})
	return ranked
}

// evidenceFromRanked 将达到最低分数的排名结果转换为最多 limit 条引用证据。
func evidenceFromRanked(ranked []rankedEntry, limit int) []Evidence {
	// evidence 是将返回给调试页的证据列表。
	evidence := make([]Evidence, 0, limit)
	// item 是当前待转换的排名知识。
	for _, item := range ranked {
		if item.score < 0.12 || len(evidence) >= limit {
			break
		}
		evidence = append(evidence, Evidence{EntryID: item.entry.ID, KnowledgeBaseID: item.entry.KnowledgeBaseID, KnowledgeBaseName: item.entry.KnowledgeBaseName, Type: item.entry.Type, Title: item.entry.Title, Excerpt: truncateRunes(item.entry.Content, 240), Score: roundScore(item.score)})
	}
	return evidence
}

// normalizeSearchText 删除 Unicode 空白和标点并转为小写，使中英文夹杂问题可稳定比较。
func normalizeSearchText(value string) string {
	// builder 累积保留的 Unicode 字母和数字。
	var builder strings.Builder
	// currentRune 是当前待判断是否保留的 Unicode 字符。
	for _, currentRune := range strings.ToLower(value) {
		if unicode.IsLetter(currentRune) || unicode.IsNumber(currentRune) {
			builder.WriteRune(currentRune)
		}
	}
	return builder.String()
}

// bigramOverlap 计算查询二元字符在候选文本中的命中比例。
func bigramOverlap(query, candidate string) float64 {
	// queryGrams 是查询文本的唯一二元字符集合。
	queryGrams := searchGrams(query)
	if len(queryGrams) == 0 {
		return 0
	}
	// candidateGrams 是候选标题与正文的二元字符集合。
	candidateGrams := searchGrams(candidate)
	// matches 统计查询二元字符在候选文本中的命中数。
	matches := 0
	// gram 是当前待查询的二元字符。
	for gram := range queryGrams {
		// exists 表示当前查询二元字符是否出现在候选文本中。
		if _, exists := candidateGrams[gram]; exists {
			matches++
		}
	}
	return float64(matches) / float64(len(queryGrams))
}

// searchGrams 将文本转换为唯一二元字符集合；单字符文本保留自身。
func searchGrams(value string) map[string]struct{} {
	// runes 是不截断 UTF-8 的 Unicode 字符序列。
	runes := []rune(value)
	// grams 保存去重后的二元字符。
	grams := make(map[string]struct{}, len(runes))
	if len(runes) == 1 {
		grams[string(runes)] = struct{}{}
		return grams
	}
	// index 是当前二元字符的开始下标。
	for index := 0; index+1 < len(runes); index++ {
		grams[string(runes[index:index+2])] = struct{}{}
	}
	return grams
}

// containsAny 判断小写文本是否包含任一稳定高风险词。
func containsAny(value string, terms []string) bool {
	// term 是当前待匹配的高风险词。
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// truncateRunes 在不截断 UTF-8 字符的前提下生成最小必要证据摘要。
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	// runes 是待截取证据文本的 Unicode 字符序列。
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}

// roundScore 将相关性分数固定到四位小数，使调试响应稳定。
func roundScore(value float64) float64 {
	return math.Round(value*10000) / 10000
}

// uniquePositiveIDs 保持首次出现顺序去除重复和非正数知识库 ID。
func uniquePositiveIDs(values []int64) []int64 {
	// result 是去重后的正数 ID 列表。
	result := make([]int64, 0, len(values))
	// seen 记录已加入的知识库 ID。
	seen := make(map[int64]struct{}, len(values))
	// value 是当前待校验和去重的知识库 ID。
	for _, value := range values {
		if value <= 0 {
			continue
		}
		// exists 表示当前知识库 ID 是否已加入去重结果。
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// currentUnix 是可在测试中替换的当前 Unix 秒时间源。
var currentUnix = func() int64 {
	return timeNowUnix()
}

// timeNowUnix 通过小函数隔离时间依赖，避免在排名逻辑中传播时钟。
func timeNowUnix() int64 {
	return unixNow()
}

// unixNow 返回当前 Unix 秒；独立变量使确定性测试可以注入时间。
var unixNow = func() int64 {
	return time.Now().Unix()
}
