#!/bin/sh

# roadmapcheck 校验动态 Roadmap 的必需结构，并阻止详细验收记录领先于状态摘要。
set -eu

# script_dir 是本脚本所在的 tools 目录；repo_root 是不依赖调用方当前目录的仓库根目录。
script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(dirname "$script_dir")

# roadmap_path 与 progress_path 允许测试使用临时夹具；生产检查默认读取仓库中的两个治理文档。
roadmap_path=${ROADMAP_PATH:-"$repo_root/ROADMAP.md"}
progress_path=${PROGRESS_PATH:-"$repo_root/docs/architecture/refactoring-progress.md"}

# fail 输出单一可操作错误并以非零状态终止，避免陈旧 Roadmap 被误判为通过。
fail() {
	printf 'roadmapcheck: %s\n' "$1" >&2
	exit 1
}

[ -f "$roadmap_path" ] || fail "缺少 ROADMAP.md：$roadmap_path"
[ -f "$progress_path" ] || fail "缺少详细验收记录：$progress_path"

# required_section 表示动态状态源必须长期保留的栏目；缺少任一栏目都无法完成交付审计。
for required_section in 文档职责 当前阶段 已完成 进行中 阻塞与风险 待办与下一步 最近验证 状态更新规则; do
	grep -Fqx "## $required_section" "$roadmap_path" || fail "ROADMAP.md 缺少栏目：## $required_section"
done

# latest_progress_heading 是详细验收记录最后一个带日期的三级维护标题，普通三级子标题不会误推进锚点。
latest_progress_heading=$(sed -n '/^### [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] /s/^### //p' "$progress_path" | tail -n 1)
[ -n "$latest_progress_heading" ] || fail "详细验收记录没有可同步的带日期 ### 维护标题"

# roadmap_markers 收集 Roadmap 中全部同步锚点；marker_count 必须严格为一，避免多个来源互相冲突。
roadmap_markers=$(sed -n 's/^验收记录同步至：`\(.*\)`。$/\1/p' "$roadmap_path")
marker_count=$(printf '%s\n' "$roadmap_markers" | sed '/^$/d' | wc -l | tr -d ' ')
[ "$marker_count" -eq 1 ] || fail "ROADMAP.md 必须且只能包含一个“验收记录同步至”锚点，当前为 $marker_count 个"

# roadmap_heading 是 Roadmap 声明已消费的最后一条详细证据，必须与真实最新标题逐字一致。
roadmap_heading=$roadmap_markers
[ "$roadmap_heading" = "$latest_progress_heading" ] || fail "ROADMAP.md 已过期：当前同步至“${roadmap_heading}”，最新记录为“${latest_progress_heading}”"

printf 'roadmapcheck: 通过（已同步至“%s”）\n' "$latest_progress_heading"
