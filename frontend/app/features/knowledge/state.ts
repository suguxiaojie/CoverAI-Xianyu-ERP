import type { KnowledgeBasePreview, KnowledgeEntryPreview, RetrievalPreview } from './types';

/** answerabilityLabels 为后端确定性检索门禁结果提供稳定中文文案。 */
export const answerabilityLabels: Readonly<Record<RetrievalPreview['answerability'], string>> = {
  answerable: '证据充分',
  needs_live_data: '需要实时数据',
  needs_human: '需要人工处理',
  insufficient: '知识不足',
};

/** filterKnowledgeBases 按名称、描述和范围过滤服务端知识库列表。 */
export const filterKnowledgeBases = (knowledgeBases: KnowledgeBasePreview[], query: string): KnowledgeBasePreview[] => {
  // normalizedQuery 是不区分英文大小写的去空格搜索文本。
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) return knowledgeBases;
  return knowledgeBases.filter(/* knowledgeBase 是当前待判断的知识库展示模型。 */ knowledgeBase => (
    `${knowledgeBase.name} ${knowledgeBase.description} ${knowledgeBase.scopeLabel}`.toLowerCase().includes(normalizedQuery)
  ));
};

/** entriesForKnowledgeBase 返回选中知识库下指定 FAQ／文档类型的条目。 */
export const entriesForKnowledgeBase = (
  entries: KnowledgeEntryPreview[],
  knowledgeBaseId: string,
  type: KnowledgeEntryPreview['type'],
): KnowledgeEntryPreview[] => entries.filter(/* entry 是当前待归属和类型检查的知识条目。 */ entry => (
  entry.knowledgeBaseId === knowledgeBaseId && entry.type === type
));
