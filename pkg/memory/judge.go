package memory

import (
	"ai-memory/pkg/llm"
	"ai-memory/pkg/types"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Judge 判定引擎：评估记忆价值和提取结构化信息
type Judge struct {
	llm          llm.LLM
	judgeModel   string
	extractModel string
}

// NewJudge 创建判定引擎实例
func NewJudge(llmInstance llm.LLM, judgeModel, extractModel string) *Judge {
	return &Judge{
		llm:          llmInstance,
		judgeModel:   judgeModel,
		extractModel: extractModel,
	}
}

// JudgeMemoryValue 判断记忆价值（单条）
func (j *Judge) JudgeMemoryValue(ctx context.Context, content string) (*types.JudgeResult, error) {
	prompt := fmt.Sprintf(`你是记忆价值评估专家。分析以下对话片段，判断是否包含值得长期记忆的信息。

对话内容：
%s

评估维度（满分1.0）：
1. 事实性 (0.4): 是否包含客观事实（如地点、日期、人名、技术栈等）
2. 偏好性 (0.3): 是否反映用户偏好（如喜好、习惯、风格等）
3. 目标性 (0.3): 是否涉及长期目标（如学习计划、项目意图等）

输出JSON格式（严格遵守，不要添加额外文本）：
{
  "value_score": 0.0-1.0,
  "confidence_score": 0.0-1.0,
  "category": "fact|preference|goal|noise",
  "reason": "简短理由",
  "tags": ["标签1", "标签2"],
  "entities": {"实体类型": "实体值"},
  "should_stage": true/false,
  "is_critical": true/false
}

判定指南：
- is_critical: 仅当满足以下任一条件时设为 true：
  1. 强烈意图/深度承诺（如“我决定要学习Golang”、“我准备搬家到上海”）
  2. 核心事实变更（如“我入职了Google”、“我结婚了”）
  3. 用户显式要求记住（如“记住，我的生日是10月1日”）
- should_stage: 通用的有价值信息。`, content)

	response, err := j.llm.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM判定失败: %w", err)
	}

	// 清理响应（可能包含markdown代码块）
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result types.JudgeResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("解析判定结果失败: %w, 原始响应: %s", err, response)
	}

	return &result, nil
}

// JudgeBatch 批量判定（降低LLM调用次数）
func (j *Judge) JudgeBatch(ctx context.Context, contents []string) ([]*types.JudgeResult, error) {
	if len(contents) == 0 {
		return nil, nil
	}

	// 构建批量prompt
	var contentList string
	for i, c := range contents {
		contentList += fmt.Sprintf("【记忆%d】\n%s\n\n", i+1, c)
	}

	prompt := fmt.Sprintf(`你是记忆价值评估专家。批量分析以下%d条对话片段，判断每条是否包含值得长期记忆的信息。

%s

评估维度（满分1.0）：
1. 事实性 (0.4): 客观事实
2. 偏好性 (0.3): 用户偏好
3. 目标性 (0.3): 长期目标

输出JSON数组格式（严格遵守，不要添加额外文本）：
[
  {
    "value_score": 0.0-1.0,
    "confidence_score": 0.0-1.0,
    "category": "fact|preference|goal|noise",
    "reason": "简短理由",
    "tags": ["标签1"],
    "entities": {"类型": "值"},
    "should_stage": true/false,
    "is_critical": true/false
  }
]

判定指南：
- is_critical: 关键事实、强烈意图或用户明确要求记忆的内容（直接晋升）。
- should_stage: 普通有价值信息（进入暂存观察）。`, len(contents), contentList)

	response, err := j.llm.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("批量判定失败: %w", err)
	}

	// 清理响应
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var results []*types.JudgeResult
	if err := json.Unmarshal([]byte(response), &results); err != nil {
		return nil, fmt.Errorf("解析批量判定结果失败: %w", err)
	}

	// 校验数量
	if len(results) != len(contents) {
		return nil, fmt.Errorf("判定结果数量不匹配: 期望%d, 实际%d", len(contents), len(results))
	}

	return results, nil
}

// ExtractStructuredTags 提取结构化标签和实体（用于LTM写入前）
func (j *Judge) ExtractStructuredTags(ctx context.Context, content string, category types.MemoryCategory) ([]string, map[string]string, error) {
	prompt := fmt.Sprintf(`提取以下记忆的结构化信息。

记忆内容：
%s

分类：%s

请提取：
1. 关键标签（2-5个简洁的中文/英文标签）
2. 实体映射（提取关键实体及其类型）

输出JSON格式：
{
  "tags": ["标签1", "标签2"],
  "entities": {"实体类型": "实体值"}
}`, content, category)

	response, err := j.llm.GenerateText(ctx, prompt)
	if err != nil {
		return nil, nil, fmt.Errorf("标签提取失败: %w", err)
	}

	// 清理响应
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result struct {
		Tags     []string          `json:"tags"`
		Entities map[string]string `json:"entities"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, nil, fmt.Errorf("解析标签提取结果失败: %w", err)
	}

	return result.Tags, result.Entities, nil
}

// SummarizeAndRestructure 将原始记忆总结为"独立可读"的事实陈述
// 输入：原始对话/事件内容
// 输出：结构化摘要（脱离上下文依然可读）
func (j *Judge) SummarizeAndRestructure(ctx context.Context, rawContent string, category types.MemoryCategory) (string, error) {
	prompt := fmt.Sprintf(`你是记忆重构专家。将以下对话/事件转换为独立的事实陈述。

原始内容：
%s

分类：%s

重构要求：
1. **独立可读**：移除"用户说"、"AI回复"等对话标记，转为客观事实
2. **第三人称**：使用"该用户"或具体人名
3. **完整信息**：包含所有关键信息（时间、地点、偏好、目标等）
4. **简洁准确**：1-3句话概括核心事实

输出格式：纯文本，不要JSON，直接输出重构后的独立事实陈述。

示例：
输入："User: 我喜欢Python\nAI: 好的，记住了"
输出："该用户偏好使用Python编程语言"`, rawContent, category)

	response, err := j.llm.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("总结重构失败: %w", err)
	}

	// 清理响应
	summary := strings.TrimSpace(response)
	summary = strings.Trim(summary, "\"") // 移除可能的引号

	if len(summary) == 0 {
		return rawContent, nil // 降级：返回原文
	}

	return summary, nil
}

// DecideMergeStrategy LLM判断两条相似记忆的合并策略
// 返回：策略类型 + 合并后内容（如适用）
func (j *Judge) DecideMergeStrategy(ctx context.Context, memory1, memory2 string) (strategy string, merged string, err error) {
	prompt := fmt.Sprintf(`你是记忆管理专家。分析两条相似的长期记忆，判断如何处理。

【记忆A】（已存在）：
%s

【记忆B】（新发现）：
%s

评估维度：
1. 信息重复度：内容是否高度重叠
2. 时间关系：是否存在信息更新/演化
3. 独立性：是否为不同时间点的独立事实

选择策略（严格输出JSON）：
{
  "strategy": "update_existing|merge|keep_both|keep_newer",
  "reason": "简短理由",
  "merged_content": "如选择merge，输出合并后的独立事实陈述"
}

策略说明：
- update_existing: 记忆B与A高度重复，只更新A的访问计数
- merge: 记忆B包含A的升级信息，合并为更完整的事实
- keep_both: 两条记忆代表不同阶段的独立事实，都保留
- keep_newer: 记忆B完全替代A，删除A保留B`, memory1, memory2)

	response, err := j.llm.GenerateText(ctx, prompt)
	if err != nil {
		return "", "", fmt.Errorf("LLM合并策略判定失败: %w", err)
	}

	// 清理响应
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result struct {
		Strategy      string `json:"strategy"`
		Reason        string `json:"reason"`
		MergedContent string `json:"merged_content"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return "", "", fmt.Errorf("解析合并策略失败: %w", err)
	}

	return result.Strategy, result.MergedContent, nil
}
