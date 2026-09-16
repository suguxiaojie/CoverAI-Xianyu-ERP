#!/bin/sh

# roadmapcheck_test 使用临时文档覆盖成功、过期、缺栏目、重复锚点和缺日期维护标题五类确定性分支。
set -eu

# script_dir 用于定位被测脚本；fixture_dir 只保存本次测试创建的临时治理文档。
script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/roadmapcheck.XXXXXX")

# cleanup 仅删除本测试自己通过 mktemp 创建的目录，并由退出陷阱统一负责。
cleanup() {
	rm -rf "$fixture_dir"
}
trap cleanup EXIT HUP INT TERM

# roadmap_path 与 progress_path 是每个用例覆盖写入的临时输入，不读取或修改真实项目文档。
roadmap_path="$fixture_dir/ROADMAP.md"
progress_path="$fixture_dir/refactoring-progress.md"

# write_valid_roadmap 生成包含全部强制栏目和一个指定同步锚点的最小合法 Roadmap。
write_valid_roadmap() {
	marker_value=$1
	printf '%s\n' \
		'# 测试 Roadmap' \
		'' \
		'## 文档职责' \
		'## 当前阶段' \
		"验收记录同步至：\`$marker_value\`。" \
		'## 已完成' \
		'## 进行中' \
		'## 阻塞与风险' \
		'## 待办与下一步' \
		'## 最近验证' \
		'## 状态更新规则' >"$roadmap_path"
}

# expect_pass 要求被测输入成功；expect_fail 要求门禁拒绝输入，防止失败分支静默退化。
expect_pass() {
	ROADMAP_PATH="$roadmap_path" PROGRESS_PATH="$progress_path" sh "$script_dir/roadmapcheck.sh" >/dev/null
}
expect_fail() {
	if ROADMAP_PATH="$roadmap_path" PROGRESS_PATH="$progress_path" sh "$script_dir/roadmapcheck.sh" >/dev/null 2>&1; then
		printf 'roadmapcheck_test: 预期失败的用例意外通过\n' >&2
		exit 1
	fi
}

# current_heading 是夹具中的最新维护标题，用于验证正常同步和陈旧锚点分支。
current_heading='2026-08-19 当前维护记录'
printf '%s\n' '# 验收记录' '' "### $current_heading" >"$progress_path"
write_valid_roadmap "$current_heading"
expect_pass

write_valid_roadmap '2026-08-19 旧维护记录'
expect_fail

write_valid_roadmap "$current_heading"
sed '/^## 阻塞与风险$/d' "$roadmap_path" >"$fixture_dir/missing-section.md"
ROADMAP_PATH="$fixture_dir/missing-section.md" PROGRESS_PATH="$progress_path" sh "$script_dir/roadmapcheck.sh" >/dev/null 2>&1 && {
	printf 'roadmapcheck_test: 缺少强制栏目的用例意外通过\n' >&2
	exit 1
}

write_valid_roadmap "$current_heading"
printf '验收记录同步至：`%s`。\n' "$current_heading" >>"$roadmap_path"
expect_fail

printf '%s\n' '# 没有日期维护标题的验收记录' '' '### 普通三级子标题' >"$progress_path"
write_valid_roadmap "$current_heading"
expect_fail

printf 'roadmapcheck_test: 通过（5 个确定性用例）\n'
