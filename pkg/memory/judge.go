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
  "should_stage": true/false
}`, content)

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
    "should_stage": true/false
  }
]`, len(contents), contentList)

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
